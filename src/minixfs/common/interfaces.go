package common

// A random access device
type BlockDevice interface {
	Read(buf interface{}, pos int64) error
	Write(buf interface{}, pos int64) error
	Close() error
}

// BlockCache is a thread-safe interface to a block cache, wrapping one or
// more block devices.
type BlockCache interface {
	// Attach a new device to the cache
	MountDevice(devno int, dev BlockDevice, info DeviceInfo) error
	// Remove a device from the cache
	UnmountDevice(devno int) error
	// Get a block from the cache
	GetBlock(dev, bnum int, btype BlockType, only_search int) *CacheBlock
	// Release (free) a block back to the cache
	PutBlock(cb *CacheBlock, btype BlockType) error
	// Invalidate all blocks for a given device
	Invalidate(dev int)
	// Flush any dirty blocks for a given device to the device
	Flush(dev int)
	// Close the block cache
	Close() error
}

type Bitmap interface {
	// Allocate a free inode, returning the number of the allocated inode
	AllocInode() (int, error)
	// Allocate a free zone, returning the number of the allocated zone. Zones
	// are numbered starting at 0, which corresponds to Firstdatazone stored
	// in the superblock. The search begins at the 'zstart' parameter, which
	// is specified absolutely (and thus is adjusted by Firstdatazone).
	AllocZone(zstart int) (int, error)
	// Free an allocated inode
	FreeInode(inum int)
	// Free an allocated zone
	FreeZone(znum int)
	// Close the bitmap server
	Close() error
}

type InodeCache interface {
	// Update the information about a given device
	MountDevice(devno int, bitmap Bitmap, info DeviceInfo)
	// Create a new inode with the given parameters
	NewInode(devno, inum int, mode, links uint16, uid int16, gid uint16, zone uint32) (*Inode, error)
	// Get an inode from the given device
	GetInode(devno, inum int) (*Inode, error)
	// Return the given inode to the cache. If the inode has been altered and
	// it has no other clients, it should be written to the block cache.
	PutInode(rip *Inode)
	// Flush the inode to the block cache, ensuring that it will be written
	// the next time the block cache is flushed.
	FlushInode(rip *Inode)
	// Returns whether or not the given device is busy. As non-busy device has
	// exactly one client of the root inode.
	IsDeviceBusy(devno int) bool
	// Close the inode cache
	Close() error
}
