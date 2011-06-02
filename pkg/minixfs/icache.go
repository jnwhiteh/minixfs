package minixfs

import (
	"os"
	"sync"
)

// The InodeCache provides a way to retrieve inodes and to cache open inodes.
// There is no explicit way of 'releasing' a cached inode, but the inode
// management functions ensure that they obtain the icache mutex before
// incrementing or decrementing the count. Since this is the only time that
// the device/number/count can change, it ensures that the cache never
// attempts to re-use an inode that is in the process of being 'acquired'.
//
// fs.put_inode
// fs.dup_inode
//
// It is likely that this is too restrictive, but it provides a nice basis for
// reasoning.
type InodeCache struct {
	// These struct elements are duplicates of those that can be found in
	// the FileSystem struct. By duplicating them, we make InodeCache a
	// self-contained data structure that has a well-defined interface.
	devs   []BlockDevice // the block devices that comprise the file system
	supers []*Superblock // the superblocks for the given devices

	fs *FileSystem // the file system that owns this cache

	inodes []*Inode // the list of in-memory inodes
	size   int

	m *sync.RWMutex
}

// Create a new InodeCache with a given size. This cache is internally
// synchronized, ensuring that the cache is only updated atomically.
func NewInodeCache(fs *FileSystem, size int) *InodeCache {
	cache := new(InodeCache)
	cache.devs = make([]BlockDevice, NR_SUPERS)
	cache.supers = make([]*Superblock, NR_SUPERS)
	cache.fs = fs

	cache.inodes = make([]*Inode, size)
	cache.size = size
	cache.m = new(sync.RWMutex)

	return cache
}

func (c *InodeCache) GetInode(dev int, num uint) (*Inode, os.Error) {
	c.m.Lock()         // acquire the write mutex
	defer c.m.Unlock() // release the write mutex

	avail := -1
	for i := 0; i < c.size; i++ {
		rip := c.inodes[i]
		if rip != nil && rip.count > 0 { // only check used slots for (dev, numb)
			if int(rip.dev) == dev && rip.inum == num {
				// this is the inode that we are looking for
				rip.count++
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
		xp = new(Inode)
	}

	super := c.supers[dev]

	// For a 4096 block size, inodes 0-63 reside in the first block
	block_offset := super.Imap_blocks + super.Zmap_blocks + 2
	block_num := ((num - 1) / super.inodes_per_block) + uint(block_offset)

	// Load the inode from the disk and create in-memory version of it
	bp := c.fs.get_block(dev, int(block_num), INODE_BLOCK)
	inodeb := bp.block.(InodeBlock)

	// We have the full block, now get the correct inode entry
	xp.disk_inode = inodeb[(num-1)%super.inodes_per_block]
	xp.dev = dev
	xp.inum = num
	xp.count = 1
	xp.dirty = false

	// add the inode to the cache
	c.inodes[avail] = xp

	return xp, nil
}

// Returns whether or not a given device is current 'busy'. A non-busy device
// will only have a single inode open, the ROOT_INODE, and it should only be
// open once.
func (c *InodeCache) IsDeviceBusy(devno int) bool {
	count := 0
	for i := 0; i < c.size; i++ {
		rip := c.inodes[i]
		if rip != nil && rip.count > 0 && rip.dev == devno {
			count += int(rip.count)
		}
	}
	return count > 1
}

// Associate a BlockDevice and *Superblock with a device number so it can be
// used internally. This operation requires the write portion of the RWMutex
// since it alters the devs and supers arrays.
func (c *InodeCache) MountDevice(devno int, dev BlockDevice, super *Superblock) os.Error {
	c.m.Lock()         // acquire the write mutex (+++)
	defer c.m.Unlock() // defer release of the write mutex (---)
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
	c.m.Lock()         // acquire the write mutex (+++)
	defer c.m.Unlock() // defer release of the write mutex (---)
	c.devs[devno] = nil
	c.supers[devno] = nil
	return nil
}
