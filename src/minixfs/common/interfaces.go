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
	GetInode(devno, inum int) (Inode, error)
	// Get a duplicate of an inode
	DupInode(rip Inode) Inode

	// Return the given inode to the cache. If the inode has been altered and
	// it has no other clients, it should be written to the block cache.
	// If the inode was previously locked, then this call will cause it to be
	// unlocked.
	PutInode(rip Inode)
	// Flush the inode to the block cache, ensuring that it will be written
	// the next time the block cache is flushed.
	FlushInode(rip LockedInode)

	// Inodes can exist in any in three different states: unlocked, read
	// locked, and write locked. Here, we use the type system to help ensure
	// that the data structures are used properly in each of the states.
	//
	// Most utility functions operate on the read-locked version of inodes, so
	// anything wishing to 'store' an inode for longer than a single operation
	// should ensure it is unlocked into an InodeId.

	// Acquire the read lock for an inode
	RLockInode(rip InodeId) Inode
	// Release the read lock for an inode
	RUnlockInode(rip Inode) InodeId

	// Acquire the exclusive lock for an inode
	WLockInode(rip Inode) LockedInode
	// Release the exclusive lock for an inode
	WUnlockInode(rip LockedInode) Inode

	// Returns whether or not the given device is busy. As non-busy device has
	// exactly one client of the root inode.
	IsDeviceBusy(devno int) bool
	// Close the inode cache
	Close() error
}

// This interface represents just the static elements of an inode that cannot
// and will not change. It is always unlocked and as a result can be stored
// (for example as the root/working directory of a process). It must be put
// back into the cache when it is no longer needed.
type InodeId interface {
	// The device number of the inode
	Devnum() int
	// The inode number of the inode
	Inum() int

	// The type masked portion of the mode
	Type() int

	// Is this inode a directory
	IsDirectory() bool
	// Is this inode a regular file
	IsRegular() bool
}

// Getters for the elements of an inode
type Inode interface {
	InodeId

	// Is this inode a mount point?
	MountPoint() bool
	// The mode
	GetMode() int
	// The number of links to this inode on the file system
	Links() int
	// Has this inode been altered since last flushed to disk
	IsDirty() bool
	// The size of the file/directory
	GetSize() int
	// Get the zone
	GetZone(znum int) uint32
}

// An interface to the more volatile fields of an inode. This can only be
// obtained by requesting a locked inode from the inode cache (or by upgrading
// an existing unlocked inode. An upgrade does not increase the count field of
// the inode, instead it converts from one into the other.
type LockedInode interface {
	Inode // include the unlocked Inode behaviour as well

	// The number of clients of this inode
	Count() int
	// Set the inode as a mount point
	SetMountPoint(bool)
	// Increment the number of links by 1
	IncLinks()
	// Decrement the number of links by 1
	DecLinks()
	// Set the mode
	SetMode(uint16)
	// Set dirty
	SetDirty(dirty bool)
	// Set zone
	SetZone(znum int, zone uint32)
}
