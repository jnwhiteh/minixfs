package icache

import (
	. "../../minixfs/common/_obj/minixfs/common"
	"os"
	"sync"
)

// A cache for inodes, alleviating the need to directly address the BlockCache
// each time an inode is required. When an inode is opened, a finode or dinode
// is spawned to receive requests. When the inode is closed with no more
// clients, then the finode/dinode is shut down.
type inodeCache struct {
	bcache  BlockCache    // the backing store for this cache
	devinfo []DeviceInfo  // information about the devices attached to the block cache
	supers  []Superblock  // a way to allocate new inodes/zones on the given device
	inodes  []*CacheInode // all cache slots

	// TODO: Need to store superblocks

	in  chan m_icache_req
	out chan m_icache_res

	// These entries could be made by wrapping *CacheInode, but we keep them
	// here just as an illustration of different ways of doing things
	waiting   [][]chan m_icache_res // wait lists for any outstanding inode load
	waiting_m *sync.Mutex           // a lock for the waiting list
}

func NewInodeCache(bcache BlockCache, numdevs int, size int) InodeCache {
	icache := &inodeCache{
		bcache,
		make([]DeviceInfo, numdevs),
		make([]Superblock, numdevs),
		make([]*CacheInode, size),
		make(chan m_icache_req),
		make(chan m_icache_res),
		make([][]chan m_icache_res, size),
		new(sync.Mutex),
	}

	for i := 0; i < len(icache.inodes); i++ {
		icache.inodes[i] = new(CacheInode)
	}

	go icache.loop()

	return icache
}

func (c *inodeCache) loop() {
	var in <-chan m_icache_req = c.in
	var out chan<- m_icache_res = c.out

	for req := range in {
		switch req := req.(type) {
		case m_icache_req_mount:
			c.devinfo[req.devno] = req.info
			c.supers[req.devno] = req.super
			out <- m_icache_res_empty{}
		case m_icache_req_getinode:
			callback := make(chan m_icache_res)

			var slot int = NO_INODE
			for i := 0; i < len(c.inodes); i++ {
				rip := c.inodes[i]
				if rip.Count > 0 {
					if rip.Devno == req.devno && rip.Inum == req.inum {
						// this is the inode we're looking for
						slot = i
						break
					}
				} else {
					slot = i // unused slot, will use if not found
				}
			}

			// Get the actual cache from the slot index
			var xp *CacheInode
			if slot > 0 && slot < len(c.inodes) {
				xp = c.inodes[slot]
			}

			if xp == nil {
				// Inode table is completely full
				out <- m_icache_res_async{callback}
				callback <- m_icache_res_getinode{nil, ENFILE}
			} else if xp.Count > 0 {
				// We found the inode, just need to return it
				out <- m_icache_res_async{callback}
				callback <- m_icache_res_getinode{xp, nil}
			} else {
				// Need to load the inode asynchronously, so make sure the
				// cache slot isn't claimed by someone else in the meantime
				xp.Devinfo = c.devinfo[req.devno]
				xp.Super = c.supers[req.devno]
				xp.Devno = req.devno
				xp.Inum = req.inum
				xp.Count++

				c.waiting_m.Lock()
				c.waiting[slot] = append(c.waiting[slot], callback)
				c.waiting_m.Unlock()

				go func() {
					c.loadInode(xp)
					c.waiting_m.Lock()
					for _, callback := range c.waiting[slot] {
						callback <- m_icache_res_getinode{xp, nil}
					}
					c.waiting[slot] = nil
					c.waiting_m.Unlock()
				}()

				out <- m_icache_res_async{callback}
			}
		case m_icache_req_putinode:
		case m_icache_req_isbusy:
			count := 0
			for i := 0; i < len(c.inodes); i++ {
				rip := c.inodes[i]
				if rip.Count > 0 && rip.Devno == req.devno {
					count += rip.Count
				}
			}
			out <- m_icache_res_isbusy{count > 1}
		case m_icache_req_close:
			busy := false
			for i := 0; i < len(c.inodes); i++ {
				if c.inodes[i].Count > 0 {
					busy = true
					break
				}
			}
			if busy {
				out <- m_icache_res_err{EBUSY}
			} else {
				out <- m_icache_res_err{nil}
				close(out)
				close(in)
			}
		}
	}
}

func (c *inodeCache) GetInode(devno, inum int) (*CacheInode, os.Error) {
	c.in <- m_icache_req_getinode{devno, inum}
	ares := (<-c.out).(m_icache_res_async)
	res := (<-ares.ch).(m_icache_res_getinode)
	return res.rip, res.err
}

func (c *inodeCache) IsDeviceBusy(devno int) bool {
	c.in <- m_icache_req_isbusy{devno}
	res := (<-c.out).(m_icache_res_isbusy)
	return res.busy
}

func (c *inodeCache) PutInode(cb *CacheInode) {
	c.in <- m_icache_req_putinode{cb}
	<-c.out
	return
}

func (c *inodeCache) Mount(devno int, super Superblock, info DeviceInfo) {
	c.in <- m_icache_req_mount{devno, super, info}
	<-c.out
	return
}

func (c *inodeCache) Close() os.Error {
	c.in <- m_icache_req_close{}
	res := (<-c.out).(m_icache_res_err)
	return res.err
}

//////////////////////////////////////////////////////////////////////////////
// Private implementations
//////////////////////////////////////////////////////////////////////////////

func (c *inodeCache) loadInode(xp *CacheInode) {
	// The count at this point is guaranteed to be > 0, so the device cannot
	// be unmounted until the load has completed and the inode has been 'put'

	info := c.devinfo[xp.Devno]
	ioffset := (xp.Inum - 1) / info.Blocksize
	blocknum := ioffset + info.MapOffset
	inodes_per_block := info.Blocksize / V2_INODE_SIZE

	// Load the inode from the disk and create an in-memory version of it
	bp := c.bcache.GetBlock(xp.Devno, blocknum, INODE_BLOCK, NORMAL)
	inodeb := bp.Block.(InodeBlock)

	// We have the full block, now get the correct inode entry
	inode_d := &inodeb[(xp.Inum-1)%inodes_per_block]
	xp.Inode = inode_d
	xp.Dirty = false
	xp.Mount = false
}

/*

func (c *inodeCache) PutInode(rip *CacheInode) {
}

func (c *inodeCache) IsDeviceBusy(devno int) bool {
	c.m.Lock()
	count := 0
	for i := 0; i < NR_INODES; i++ {
		count += c.inodes[i].Count
	}
	c.m.Unlock()

	return count != 1
}

/*
// An entry in the inode table is to be written to disk (via the buffer cache)
func (c *InodeCache) WriteInode(rip *CacheInode) {
	// Get the super-block on which the inode resides
	super := c.supers[rip.dev]

	// For a 4096 block size, inodes 0-63 reside in the first block
	block_offset := super.Imap_blocks + super.Zmap_blocks + 2
	block_num := ((rip.inum - 1) / super.inodes_per_block) + uint(block_offset)

	// Load the inode from the disk and create in-memory version of it
	bp := c.bcache.GetBlock(rip.dev, int(block_num), INODE_BLOCK, NORMAL)
	inodeb := bp.block.(InodeBlock)

	// TODO: Update times, handle read-only superblocks
	bp.dirty = true

	// Copy the disk_inode from rip into the inode block
	inodeb[(rip.inum-1)%super.inodes_per_block] = *rip.disk
	rip.SetDirty(false)
	c.bcache.PutBlock(bp, INODE_BLOCK)
}

// Returns whether or not a given device is current 'busy'. A non-busy device
// will only have a single inode open, the ROOT_INODE, and it should only be
// open once.
func (c *InodeCache) IsDeviceBusy(devno int) bool {
	// Acquire the icache mutex
	c.m.RLock()
	defer c.m.RUnlock()

	count := 0
	for i := 0; i < c.size; i++ {
		rip := c.inodes[i]
		if rip != nil && rip.Count() > 0 && rip.dev == devno {
			count += int(rip.Count())
		}
	}
	return count > 1
}

// Associate a BlockDevice and *Superblock with a device number so it can be
// used internally. This operation requires the write portion of the RWMutex
// since it alters the devs and supers arrays.
func (c *InodeCache) MountDevice(devno int, dev RandDevice, super *Superblock) os.Error {
	if c.devs[devno] != nil || c.supers[devno] != nil {
		return EBUSY
	}
	c.devs[devno] = dev
	c.supers[devno] = super
	return nil
}

// Clear an association between a BlockDevice/*Superblock pair and a device
// number.
func (c *InodeCache) UnmountDevice(devno int) os.Error {
	c.devs[devno] = nil
	c.supers[devno] = nil
	return nil
}

*/
