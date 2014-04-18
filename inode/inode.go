package inode

import (
	"github.com/jnwhiteh/minixfs/common"
	"sync"
)

type server_InodeTbl struct {
	bcache  common.BlockCache
	devices []*common.DeviceInfo
	inodes  []*common.Inode

	in  chan reqInodeTbl
	out chan resInodeTbl

	waiting   [][]chan resInodeTbl
	m_waiting *sync.Mutex
}

func NewCache(bcache common.BlockCache, numdevs int, size int) common.InodeTbl {
	cache := &server_InodeTbl{
		bcache,
		make([]*common.DeviceInfo, numdevs),
		make([]*common.Inode, size),
		make(chan reqInodeTbl),
		make(chan resInodeTbl),
		make([][]chan resInodeTbl, size),
		new(sync.Mutex),
	}

	for i := 0; i < len(cache.inodes); i++ {
		inode := new(common.Inode)
		inode.Bcache = bcache
		inode.Icache = cache
		cache.inodes[i] = inode
	}

	go cache.loop()

	return cache
}

func (itable *server_InodeTbl) loop() {
	alive := true
	for alive {
		req := <-itable.in
		switch req := req.(type) {
		case req_InodeTbl_MountDevice:
			itable.devices[req.devnum] = req.info
			itable.out <- res_InodeTbl_MountDevice{}
			// Code here
		case req_InodeTbl_UnmountDevice:
			// TODO: Do something more here?
			itable.devices[req.devnum] = nil
			itable.out <- res_InodeTbl_UnmountDevice{}
		case req_InodeTbl_GetInode:
			callback := make(chan resInodeTbl)

			slot := itable.findSlot(req.devnum, req.inum)
			var xp *common.Inode
			if slot != common.NO_INODE && slot < len(itable.inodes) {
				xp = itable.inodes[slot]
			}

			if xp == nil {
				// Inode table is completely full
				itable.out <- res_InodeTbl_Async{callback}
				callback <- res_InodeTbl_GetInode{nil, common.ENFILE}
			} else if xp.Count > 0 {
				// We found the inode, just need to return it
				xp.Count++
				itable.out <- res_InodeTbl_Async{callback}
				callback <- res_InodeTbl_GetInode{xp, nil}
			} else {
				// Need to load the inode asynchronously, so make sure the
				// cache slot isn't claimed by someone else in the meantime
				xp.Devinfo = itable.devices[req.devnum]
				xp.Inum = req.inum
				xp.Count++

				// Aquire the waiting lock and add us to the wait list
				itable.m_waiting.Lock()
				itable.waiting[slot] = append(itable.waiting[slot], callback)
				itable.m_waiting.Unlock()

				go func() {
					// Load the inode into the Inode
					itable.loadInode(xp)
					itable.m_waiting.Lock()
					for _, callback := range itable.waiting[slot] {
						callback <- res_InodeTbl_GetInode{xp, nil}
					}
					itable.waiting[slot] = nil
					itable.m_waiting.Unlock()
				}()

				itable.out <- res_InodeTbl_Async{callback}
			}
		case req_InodeTbl_DupInode:
			// Given an inode, duplicate it by incrementing its count
			rip := req.inode
			rip.Count++
			itable.out <- res_InodeTbl_DupInode{rip}
		case req_InodeTbl_PutInode:
			rip := req.inode

			if rip == nil {
				itable.out <- res_InodeTbl_PutInode{}
				continue
			}

			rip.Count--
			if rip.Count == 0 { // means no one is using it now
				if rip.Nlinks == 0 { // free the inode
					common.Truncate(rip, 0, itable.bcache) // return all the disk blocks
					rip.Mode = common.I_NOT_ALLOC
					rip.Dirty = true
					rip.Devinfo.AllocTbl.FreeInode(rip.Inum)
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
					itable.writeInode(rip)
				}
			}

			itable.out <- res_InodeTbl_PutInode{}
		case req_InodeTbl_FlushInode:
			rip := req.inode

			if rip == nil {
				itable.out <- res_InodeTbl_FlushInode{}
			} else {
				itable.writeInode(rip)
			}
			itable.out <- res_InodeTbl_FlushInode{}
		case req_InodeTbl_IsDeviceBusy:
			count := 0
			for i := 0; i < len(itable.inodes); i++ {
				rip := itable.inodes[i]
				if rip.Count > 0 && rip.Devinfo.Devnum == req.devnum {
					count += rip.Count
				}
			}
			itable.out <- res_InodeTbl_IsDeviceBusy{count > 1}
		case req_InodeTbl_Shutdown:
			for i := 0; i < len(itable.devices); i++ {
				if itable.devices[i] != nil {
					itable.out <- res_InodeTbl_Shutdown{common.EBUSY}
					continue
				}
			}
			itable.out <- res_InodeTbl_Shutdown{nil}
			alive = false
		}
	}
}

// Returns the slot that contains a given inode, an available slot is the
// given inode is not present, or NO_INODE.
func (c *server_InodeTbl) findSlot(devnum, inum int) int {
	var slot int = common.NO_INODE

	for i := 0; i < len(c.inodes); i++ {
		rip := c.inodes[i]
		if rip.Count > 0 {
			if rip.Devinfo.Devnum == devnum && rip.Inum == inum {
				// this is the inode we're looking for
				return i
			}
		} else {
			slot = i // unused slot, will use if not found
		}
	}

	return slot
}

func (c *server_InodeTbl) loadInode(xp *common.Inode) {
	// The count at this point is guaranteed to be > 0, so the device cannot
	// be unmounted until the load has completed and the inode has been 'put'

	inum := xp.Inum - 1

	info := xp.Devinfo

	inodes_per_block := info.Blocksize / common.V2_INODE_SIZE
	ioffset := inum % inodes_per_block
	blocknum := info.MapOffset + (inum / inodes_per_block)

	// Load the inode from the disk and create an in-memory version of it
	bp := c.bcache.GetBlock(info.Devnum, blocknum, common.INODE_BLOCK, common.NORMAL)
	inodeb := bp.Block.(common.InodeBlock)

	// We have the full block, now get the correct inode entry
	inode_d := &inodeb[ioffset]
	xp.Disk_Inode = inode_d
	xp.Dirty = false
	xp.Mounted = nil
}

func (c *server_InodeTbl) writeInode(xp *common.Inode) {
	// Calculate the block number we need
	inum := xp.Inum - 1
	info := xp.Devinfo
	inodes_per_block := info.Blocksize / common.V2_INODE_SIZE
	ioffset := inum % inodes_per_block
	block_num := info.MapOffset + (inum / inodes_per_block)

	// Load the inode from the disk
	bp := c.bcache.GetBlock(info.Devnum, block_num, common.INODE_BLOCK, common.NORMAL)
	inodeb := bp.Block.(common.InodeBlock)

	// TODO: Update times, handle read-only filesystems
	bp.Dirty = true

	// Copy the disk_inode from rip into the inode block
	inodeb[ioffset] = *xp.Disk_Inode
	xp.Dirty = false
	c.bcache.PutBlock(bp, common.INODE_BLOCK)
}
