package minixfs

import (
	"os"
	"sync"
)

// The InodeCache provides a way to retrieve inodes and to cache open inodes.
// There is no explicit way of 'releasing' a cached inode, but an inode with 0
// count may be re-used.
type InodeCache struct {
	// These struct elements are duplicates of those that can be found in the
	// FileSystem struct. By duplicating them, we make InodeCache a
	// self-contained data structure that has a well-defined interface.
	devs   []BlockDevice // the block devices that comprise the file system
	supers []*Superblock // the superblocks for the given devices

	bcache BlockCache

	inodes []*Inode // the list of in-memory inodes
	size   int

	m *sync.RWMutex
}

// Create a new InodeCache with a given size. This cache is internally
// synchronized, ensuring that the cache is only updated atomically.
func NewInodeCache(bcache BlockCache, size int) *InodeCache {
	cache := new(InodeCache)
	cache.devs = make([]BlockDevice, NR_SUPERS)
	cache.supers = make([]*Superblock, NR_SUPERS)
	cache.bcache = bcache

	cache.inodes = make([]*Inode, size)
	cache.size = size

	cache.m = new(sync.RWMutex)

	return cache
}

func (c *InodeCache) GetInode(dev int, num uint) (*Inode, os.Error) {
	// Acquire the mutex so we can alter the inode cache
	c.m.Lock()
	defer c.m.Unlock()

	avail := -1
	for i := 0; i < c.size; i++ {
		rip := c.inodes[i]
		if rip != nil && rip.Count() > 0 { // only check used slots for (dev, numb)
			if int(rip.dev) == dev && rip.inum == num {
				// this is the inode that we are looking for
				rip.IncCount()
				return rip, nil
			}
		} else {
			avail = i // remember this free slot for late
		}
	}

	// Inode we want is not currently in ise. Did we find a free slot?
	if avail == -1 { // inode table completely full
		return nil, ENFILE
	}

	// A free inode slot has been located. Load the inode into it
	xp := c.inodes[avail]
	if xp == nil {
		xp = NewInode()
	}

	super := c.supers[dev]

	// For a 4096 block size, inodes 0-63 reside in the first block
	block_offset := super.Imap_blocks + super.Zmap_blocks + 2
	block_num := ((num - 1) / super.inodes_per_block) + uint(block_offset)

	// Load the inode from the disk and create in-memory version of it
	bp := c.bcache.GetBlock(dev, int(block_num), INODE_BLOCK, NORMAL)
	inodeb := bp.block.(InodeBlock)

	// We have the full block, now get the correct inode entry
	inode_d := &inodeb[(num-1)%super.inodes_per_block]
	xp.disk = inode_d
	xp.super = super
	xp.dev = dev
	xp.inum = num
	xp.SetCount(1)
	xp.SetDirty(false)

	// add the inode to the cache
	c.inodes[avail] = xp

	return xp, nil
}

// An entry in the inode table is to be written to disk (via the buffer cache)
func (c *InodeCache) WriteInode(rip *Inode) {
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
func (c *InodeCache) MountDevice(devno int, dev BlockDevice, super *Superblock) os.Error {
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
