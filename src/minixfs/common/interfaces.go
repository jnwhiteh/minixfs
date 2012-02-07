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
	// Get an inode from the given device
	GetInode(devno, inum int) (*CacheInode, error)
	// Return the given inode to the cache. If the inode has been altered and
	// it has no other clients, it should be written to the block cache.
	PutInode(rip *CacheInode)
	// Flush the inode to the block cache, ensuring that it will be written
	// the next time the block cache is flushed.
	FlushInode(rip *CacheInode)
	// Returns whether or not the given device is busy. As non-busy device has
	// exactly one client of the root inode.
	IsDeviceBusy(devno int) bool
	// Close the inode cache
	Close() error
}

type Finode interface {
	// Read up to len(buf) bytes from pos within the file. Return the number
	// of bytes actually read and any error that may have occurred.
	Read(buf []byte, pos int) (int, error)
	// Write len(buf) bytes from buf to the given position in the file. Return
	// the number of bytes actually written and any error that may have
	// occurred.
	Write(buf []byte, pos int) (int, error)
	// Close the finode
	Close() error
}

type Dinode interface {
	// Search the directory for an entry named 'name' and return the
	// devno/inum of the inode, if found.
	Lookup(name string) (bool, int, int)
	// Search the directory for an entry named 'name' and fetch the
	// inode from the given InodeCache.
	LookupGet(name string, icache InodeCache) (*CacheInode, error)
	// Add an entry 'name' to the directory listing, pointing to the 'inum'
	// inode.
	Link(name string, inum int) error
	// Remove the entry named 'name' from the directory listing.
	Unlink(name string) error
	// Returns whether or not the directory is empty (i.e. only contains . and
	// .. entries).
	IsEmpty() bool
	// close the dinode
	Close() error
}
