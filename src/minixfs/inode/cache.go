package inode

import (
	. "minixfs/common"
	"minixfs/utils"
	"sync"
)

// A cache for inodes, alleviating the need to directly address the BlockCache
// each time an inode is required. When an inode is opened, a finode or dinode
// is spawned to receive requests. When the inode is closed with no more
// clients, then the finode/dinode is shut down.
type inodeCache struct {
	bcache  BlockCache   // the backing store for this cache
	devinfo []DeviceInfo // information about the devices attached to the block cache
	bitmaps []Bitmap     // a way to allocate new inodes/zones on the given device
	inodes  []*Inode     // all cache slots

	in  chan m_icache_req
	out chan m_icache_res

	// These entries could be made by wrapping *Inode, but we keep them
	// here just as an illustration of different ways of doing things
	waiting   [][]chan m_icache_res // wait lists for any outstanding inode load
	waiting_m *sync.Mutex           // a lock for the waiting list
}

func NewCache(bcache BlockCache, numdevs int, size int) InodeCache {
	cache := &inodeCache{
		bcache,
		make([]DeviceInfo, numdevs),
		make([]Bitmap, numdevs),
		make([]*Inode, size),
		make(chan m_icache_req),
		make(chan m_icache_res),
		make([][]chan m_icache_res, size),
		new(sync.Mutex),
	}

	for i := 0; i < len(cache.inodes); i++ {
		inode := new(Inode)
		inode.Bcache = bcache
		inode.Icache = cache
		inode.RWMutex = new(sync.RWMutex)
		cache.inodes[i] = inode
	}

	go cache.loop()

	return cache
}

func (c *inodeCache) loop() {
	var in <-chan m_icache_req = c.in
	var out chan<- m_icache_res = c.out

	for req := range in {
		switch req := req.(type) {
		case m_icache_req_mount:
			c.devinfo[req.devno] = req.info
			c.bitmaps[req.devno] = req.bmap
			out <- m_icache_res_empty{}
		case m_icache_req_newinode:
			// TODO: Refactor this so there's no duplication
			callback := make(chan m_icache_res)

			var slot int = NO_INODE
			for i := 0; i < len(c.inodes); i++ {
				rip := c.inodes[i]
				if rip.Count > 0 {
					if rip.Devnum == req.devno && rip.Inum == req.inum {
						// this is the inode we're looking for
						slot = i
						break
					}
				} else {
					slot = i // unused slot, will use if not found
				}
			}

			// Get the actual cache from the slot index
			var xp *Inode
			if slot > 0 && slot < len(c.inodes) {
				xp = c.inodes[slot]
			}

			if xp == nil {
				// Inode table is completely full
				out <- m_icache_res_async{callback}
				callback <- m_icache_res_newinode{nil, ENFILE}
			} else if xp.Count > 0 {
				// We found the inode, just need to return it
				xp.Count++
				out <- m_icache_res_async{callback}
				callback <- m_icache_res_newinode{xp, nil}
			} else {
				// Need to load the inode asynchronously, so make sure the
				// cache slot isn't claimed by someone else in the meantime
				xp.Devinfo = c.devinfo[req.devno]
				xp.Bitmap = c.bitmaps[req.devno]
				xp.Devnum = req.devno
				xp.Inum = req.inum
				xp.Count++

				c.waiting_m.Lock()
				c.waiting[slot] = append(c.waiting[slot], callback)
				c.waiting_m.Unlock()

				go func() {
					// Load the inode into the Inode
					c.loadInode(xp)
					// TODO: THIS IS THE CHANGE HERE
					// Fill in the specified parameters
					xp.Mode = req.mode
					xp.Nlinks = req.links
					xp.Uid = req.uid
					xp.Gid = req.gid
					xp.Zone[0] = req.zone

					c.waiting_m.Lock()
					for _, callback := range c.waiting[slot] {
						callback <- m_icache_res_newinode{xp, nil}
					}
					c.waiting[slot] = nil
					c.waiting_m.Unlock()
				}()

				out <- m_icache_res_async{callback}
			}
		case m_icache_req_getinode:
			callback := make(chan m_icache_res)

			var slot int = NO_INODE
			for i := 0; i < len(c.inodes); i++ {
				rip := c.inodes[i]
				if rip.Count > 0 {
					if rip.Devnum == req.devno && rip.Inum == req.inum {
						// this is the inode we're looking for
						slot = i
						break
					}
				} else {
					slot = i // unused slot, will use if not found
				}
			}

			// Get the actual cache from the slot index
			var xp *Inode
			if slot > 0 && slot < len(c.inodes) {
				xp = c.inodes[slot]
			}

			if xp == nil {
				// Inode table is completely full
				out <- m_icache_res_async{callback}
				callback <- m_icache_res_getinode{nil, ENFILE}
			} else if xp.Count > 0 {
				// We found the inode, just need to return it
				xp.Count++
				out <- m_icache_res_async{callback}
				callback <- m_icache_res_getinode{xp, nil}
			} else {
				// Need to load the inode asynchronously, so make sure the
				// cache slot isn't claimed by someone else in the meantime
				xp.Devinfo = c.devinfo[req.devno]
				xp.Bitmap = c.bitmaps[req.devno]
				xp.Devnum = req.devno
				xp.Inum = req.inum
				xp.Count++

				c.waiting_m.Lock()
				c.waiting[slot] = append(c.waiting[slot], callback)
				c.waiting_m.Unlock()

				go func() {
					// Load the inode into the Inode
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
			// TODO: Is this function correct?
			rip := req.rip
			if rip == nil {
				return
			}

			rip.Count--
			if rip.Count == 0 { // means no one is using it now
				if rip.Nlinks == 0 { // free the inode
					utils.Truncate(rip, rip.Bitmap, c.bcache) // return all the disk blocks
					rip.Mode = I_NOT_ALLOC
					rip.Dirty = true
					rip.Bitmap.FreeInode(rip.Inum)
				} else {
					// TODO: Handle the pipe case here
					// if rip.pipe == true {
					//   truncate(rip)
					// }
				}
				// rip.pipe = false

				if rip.Dirty {
					// Write this inode out to disk
					// TODO: Should this be performed asynchronously?
					c.writeInode(rip)
				}
			}

			out <- m_icache_res_empty{}
		case m_icache_req_flushinode:
			c.writeInode(req.rip)
			out <- m_icache_res_empty{}
		case m_icache_req_isbusy:
			count := 0
			for i := 0; i < len(c.inodes); i++ {
				rip := c.inodes[i]
				if rip.Count > 0 && rip.Devnum == req.devno {
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
				close(c.out)
				close(c.in)
			}
		}
	}
}

func (c *inodeCache) NewInode(devno, inum int, mode, links uint16, uid int16, gid uint16, zone uint32) (*Inode, error) {
	c.in <- m_icache_req_newinode{devno, inum, mode, links, uid, gid, zone}
	ares := (<-c.out).(m_icache_res_async)
	res := (<-ares.ch).(m_icache_res_newinode)
	return res.rip, res.err
}

func (c *inodeCache) GetInode(devno, inum int) (*Inode, error) {
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

func (c *inodeCache) PutInode(cb *Inode) {
	c.in <- m_icache_req_putinode{cb}
	<-c.out
	return
}

func (c *inodeCache) FlushInode(cb *Inode) {
	c.in <- m_icache_req_flushinode{cb}
	<-c.out
	return
}

func (c *inodeCache) MountDevice(devno int, bmap Bitmap, info DeviceInfo) {
	c.in <- m_icache_req_mount{devno, bmap, info}
	<-c.out
	return
}

func (c *inodeCache) Close() error {
	c.in <- m_icache_req_close{}
	res := (<-c.out).(m_icache_res_err)
	return res.err
}

//////////////////////////////////////////////////////////////////////////////
// Private implementations
//////////////////////////////////////////////////////////////////////////////

func (c *inodeCache) loadInode(xp *Inode) {
	// The count at this point is guaranteed to be > 0, so the device cannot
	// be unmounted until the load has completed and the inode has been 'put'

	inum := xp.Inum - 1
	info := c.devinfo[xp.Devnum]
	inodes_per_block := info.Blocksize / V2_INODE_SIZE
	ioffset := inum % inodes_per_block
	blocknum := info.MapOffset + (inum / inodes_per_block)

	// Load the inode from the disk and create an in-memory version of it
	bp := c.bcache.GetBlock(xp.Devnum, blocknum, INODE_BLOCK, NORMAL)
	inodeb := bp.Block.(InodeBlock)

	// We have the full block, now get the correct inode entry
	inode_d := &inodeb[ioffset]
	xp.Disk_Inode = inode_d
	xp.Dirty = false
	xp.Mount = false
}

func (c *inodeCache) writeInode(xp *Inode) {
	// Calculate the block number we need
	inum := xp.Inum - 1
	info := c.devinfo[xp.Devnum]
	inodes_per_block := info.Blocksize / V2_INODE_SIZE
	ioffset := inum % inodes_per_block
	block_num := info.MapOffset + (inum / inodes_per_block)

	// Load the inode from the disk
	bp := c.bcache.GetBlock(xp.Devnum, block_num, INODE_BLOCK, NORMAL)
	inodeb := bp.Block.(InodeBlock)

	// TODO: Update times, handle read-only filesystems
	bp.Dirty = true

	// Copy the disk_inode from rip into the inode block
	inodeb[ioffset] = *xp.Disk_Inode
	xp.Dirty = false
	c.bcache.PutBlock(bp, INODE_BLOCK)
}

var _ InodeCache = &inodeCache{}
