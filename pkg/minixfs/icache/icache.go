package minixfs

import (
	. "minixfs/common"
	"os"
	"sync"
)

// A cache for inodes, alleviating the need to directly address the BlockCache
// each time an inode is required.
type inodeCache struct {
	bcache  BlockCache
	devinfo []DeviceInfo
	inodes  []*CacheInode
	m       *sync.Mutex
}

func NewInodeCache(bcache BlockCache) InodeCache {
	icache := &inodeCache{
		bcache,
		make([]DeviceInfo, NR_INODES),
		make([]*CacheInode, NR_INODES),
		new(sync.Mutex),
	}

	for i := 0; i < NR_INODES; i++ {
		icache.inodes[i] = new(CacheInode)
	}

	return icache
}

func (c *inodeCache) UpdateDeviceInfo(devno int, info DeviceInfo) {
	c.m.Lock()
	c.devinfo[devno] = info
	c.m.Unlock()
}

func (c *inodeCache) GetInode(devno, inum int) (*CacheInode, os.Error) {
	var xp *CacheInode

	// Find an available slot
	c.m.Lock()
	for i := 0; i < NR_INODES; i++ {
		rip := c.inodes[i]
		if rip.Count > 0 {
			// only check used slots for (devno, inum)
			if rip.Devno == devno && rip.Inum == inum {
				// this is the inode that we're looking for
				rip.Count++
				return rip, nil
			}
		} else {
			xp = rip // remember this free slot for later
		}
	}

	// Claim this block so it isn't re-used
	if xp != nil {
		xp.Count++
		xp.Devno = devno
		xp.Inum = inum
	}
	c.m.Unlock()

	// Is the inode table completely full?
	if xp == nil {
		return nil, ENFILE
	}

	// The device cannot be unmounted at this point, because this inode will
	// appear busy.
	info := c.devinfo[devno]
	ioffset := (inum - 1) / info.Blocksize
	blocknum := ioffset + info.MapOffset
	inodes_per_block := info.Blocksize / V2_INODE_SIZE

	// Load the inode from the disk and create an in-memory version of it
	bp := c.bcache.GetBlock(devno, blocknum, INODE_BLOCK, NORMAL)
	inodeb := bp.Block.(InodeBlock)

	// We have the full block, now get the correct inode entry
	inode_d := &inodeb[(inum-1)%inodes_per_block]
	xp.Inode = inode_d
	xp.Devno = devno
	xp.Inum = inum
	xp.Dirty = false
	xp.Mount = false

	return xp, nil
}

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
