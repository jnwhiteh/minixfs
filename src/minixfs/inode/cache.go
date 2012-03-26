package inode

import (
	. "minixfs/common"
	"sync"
)

// A cache for inodes, alleviating the need to directly address the BlockCache
// each time an inode is required. When an inode is opened, a finode or dinode
// is spawned to receive requests. When the inode is closed with no more
// clients, then the finode/dinode is shut down.
type inodeCache struct {
	bcache  BlockCache    // the backing store for this cache
	devinfo []DeviceInfo  // information about the devices attached to the block cache
	bitmaps []Bitmap      // a way to allocate new inodes/zones on the given device
	inodes  []*cacheInode // all cache slots

	in  chan reqInodeCache
	out chan resInodeCache

	// These entries could be made by wrapping *cacheInode, but we keep them
	// here just as an illustration of different ways of doing things
	waiting   [][]chan resInodeCache // wait lists for any outstanding inode load
	waiting_m *sync.Mutex            // a lock for the waiting list
}

func NewCache(bcache BlockCache, numdevs int, size int) InodeCache {
	cache := &inodeCache{
		bcache,
		make([]DeviceInfo, numdevs),
		make([]Bitmap, numdevs),
		make([]*cacheInode, size),
		make(chan reqInodeCache),
		make(chan resInodeCache),
		make([][]chan resInodeCache, size),
		new(sync.Mutex),
	}

	for i := 0; i < len(cache.inodes); i++ {
		inode := new(cacheInode)
		inode.bcache = bcache
		inode.icache = cache
		inode.RWMutex = new(sync.RWMutex)
		cache.inodes[i] = inode
	}

	go cache.loop()

	return cache
}

func (c *inodeCache) loop() {
	alive := true
	for alive {
		req := <-c.in
		switch req := req.(type) {

		case req_InodeCache_MountDevice:
			c.devinfo[req.devno] = req.info
			c.bitmaps[req.devno] = req.bitmap
			c.out <- res_InodeCache_MountDevice{}
		case req_InodeCache_GetInode:
			callback := make(chan resInodeCache)

			slot := c.findSlot(req.devno, req.inum)
			var xp *cacheInode
			if slot != NO_INODE && slot < len(c.inodes) {
				xp = c.inodes[slot]
			}

			if xp == nil {
				// Inode table is completely full
				c.out <- res_InodeCache_Async{callback}
				callback <- res_InodeCache_GetInode{nil, ENFILE}
			} else if xp.count > 0 {
				// We found the inode, just need to return it
				xp.count++
				xp.RLock()
				c.out <- res_InodeCache_Async{callback}
				callback <- res_InodeCache_GetInode{xp, nil}
			} else {
				// Need to load the inode asynchronously, so make sure the
				// cache slot isn't claimed by someone else in the meantime
				xp.devinfo = c.devinfo[req.devno]
				xp.bitmap = c.bitmaps[req.devno]
				xp.devnum = req.devno
				xp.inum = req.inum
				xp.count++

				c.waiting_m.Lock()
				c.waiting[slot] = append(c.waiting[slot], callback)
				c.waiting_m.Unlock()

				go func() {
					// Load the inode into the Inode
					c.loadInode(xp)
					c.waiting_m.Lock()
					for _, callback := range c.waiting[slot] {
						xp.RLock() // read lock the inode
						callback <- res_InodeCache_GetInode{xp, nil}
					}
					c.waiting[slot] = nil
					c.waiting_m.Unlock()
				}()

				c.out <- res_InodeCache_Async{callback}
			}
		case req_InodeCache_DupInode:
			// We have an inode, increment its count, RLock it and return it
			rip := req.rip.(*cacheInode)

			callback := make(chan resInodeCache)
			rip.count++
			go func() {
				rip.RLock() // read lock the inode
				callback <- res_InodeCache_DupInode{rip}
			}()

			c.out <- res_InodeCache_Async{callback}

		case req_InodeCache_RLockInode:
			// Lock an InodeId into an Inode
			rip := req.rip.(*cacheInode)

			callback := make(chan resInodeCache)
			go func() {
				rip.RLock()
				callback <- res_InodeCache_RLockInode{rip}
			}()

			c.out <- res_InodeCache_Async{callback}

		case req_InodeCache_RUnlockInode:
			// Unlock an Inode into an InodeId
			rip := req.rip.(*cacheInode)

			callback := make(chan resInodeCache)
			go func() {
				rip.RUnlock()
				callback <- res_InodeCache_RUnlockInode{rip}
			}()

			c.out <- res_InodeCache_Async{callback}

		case req_InodeCache_WLockInode:
			// Lock an Inode into a LockedInode
			rip := req.rip.(*cacheInode)

			callback := make(chan resInodeCache)
			go func() {
				rip.RUnlock() // it should be read-locked right now
				rip.Lock()
				rip.locked = true
				callback <- res_InodeCache_WLockInode{rip}
			}()

			c.out <- res_InodeCache_Async{callback}

		case req_InodeCache_WUnlockInode:
			// Unlock a LockedInode into an Inode
			rip := req.rip.(*cacheInode)

			callback := make(chan resInodeCache)
			go func() {
				rip.locked = false
				rip.Unlock()
				rip.RLock()
				callback <- res_InodeCache_WUnlockInode{rip}
			}()

			c.out <- res_InodeCache_Async{callback}

		case req_InodeCache_PutInode:
			rip := req.rip.(*cacheInode)

			if rip == nil {
				c.out <- res_InodeCache_PutInode{}
				continue
			}

			rip.count--
			if rip.count == 0 { // means no one is using it now
				if rip.Nlinks == 0 { // free the inode
					Truncate(rip, rip.bitmap, c.bcache) // return all the disk blocks
					rip.Mode = I_NOT_ALLOC
					rip.dirty = true
					rip.bitmap.FreeInode(rip.inum)
				} else {
					// TODO: Handle the pipe case here
					// if rip.pipe == true {
					//   truncate(rip)
					// }
				}
				// rip.pipe = false

				if rip.dirty {
					// Write this inode out to disk
					// TODO: Should this be performed asynchronously?
					c.writeInode(rip)
				}
			}

			// At this point the inode is clean and ready to go, so release it
			// to the rest of the world
			if rip.locked {
				rip.locked = false
				rip.Unlock()
			} else {
				rip.RUnlock()
			}

			c.out <- res_InodeCache_PutInode{}

		case req_InodeCache_FlushInode:
			rip := req.rip.(*cacheInode)

			if rip == nil {
				c.out <- res_InodeCache_FlushInode{}
				continue
			}

			c.writeInode(rip)
			c.out <- res_InodeCache_FlushInode{}

		case req_InodeCache_IsDeviceBusy:
			count := 0
			for i := 0; i < len(c.inodes); i++ {
				rip := c.inodes[i]
				if rip.count > 0 && rip.devnum == req.devno {
					count += rip.count
				}
			}
			c.out <- res_InodeCache_IsDeviceBusy{count > 1}

		case req_InodeCache_Close:
			busy := false
			for i := 0; i < len(c.inodes); i++ {
				if c.inodes[i].count > 0 {
					busy = true
					break
				}
			}
			if busy {
				c.out <- res_InodeCache_Close{EBUSY}
			} else {
				c.out <- res_InodeCache_Close{nil}
				alive = false
			}
		}
	}
}

//////////////////////////////////////////////////////////////////////////////
// Private implementations
//////////////////////////////////////////////////////////////////////////////

// Returns the slot that contains a given inode, an available slot is the
// given inode is not present, or NO_INODE.
func (c *inodeCache) findSlot(devnum, inum int) int {
	var slot int = NO_INODE

	for i := 0; i < len(c.inodes); i++ {
		rip := c.inodes[i]
		if rip.count > 0 {
			if rip.devnum == devnum && rip.inum == inum {
				// this is the inode we're looking for
				return i
			}
		} else {
			slot = i // unused slot, will use if not found
		}
	}

	return slot
}

func (c *inodeCache) loadInode(xp *cacheInode) {
	// The count at this point is guaranteed to be > 0, so the device cannot
	// be unmounted until the load has completed and the inode has been 'put'

	inum := xp.inum - 1
	info := c.devinfo[xp.devnum]
	inodes_per_block := info.Blocksize / V2_INODE_SIZE
	ioffset := inum % inodes_per_block
	blocknum := info.MapOffset + (inum / inodes_per_block)

	// Load the inode from the disk and create an in-memory version of it
	bp := c.bcache.GetBlock(xp.devnum, blocknum, INODE_BLOCK, NORMAL)
	inodeb := bp.Block.(InodeBlock)

	// We have the full block, now get the correct inode entry
	inode_d := &inodeb[ioffset]
	xp.Disk_Inode = inode_d
	xp.dirty = false
	xp.mount = false
}

func (c *inodeCache) writeInode(xp *cacheInode) {
	// Calculate the block number we need
	inum := xp.inum - 1
	info := c.devinfo[xp.devnum]
	inodes_per_block := info.Blocksize / V2_INODE_SIZE
	ioffset := inum % inodes_per_block
	block_num := info.MapOffset + (inum / inodes_per_block)

	// Load the inode from the disk
	bp := c.bcache.GetBlock(xp.devnum, block_num, INODE_BLOCK, NORMAL)
	inodeb := bp.Block.(InodeBlock)

	// TODO: Update times, handle read-only filesystems
	bp.Dirty = true

	// Copy the disk_inode from rip into the inode block
	inodeb[ioffset] = *xp.Disk_Inode
	xp.dirty = false
	c.bcache.PutBlock(bp, INODE_BLOCK)
}

var _ InodeCache = &inodeCache{}
