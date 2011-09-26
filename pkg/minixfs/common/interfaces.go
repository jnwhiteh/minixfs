package common

import (
	"os"
)

// A random access device
type RandDevice interface {
	Read(buf interface{}, pos int64) os.Error
	Write(buf interface{}, pos int64) os.Error
	Close() os.Error
}

// BlockCache is a thread-safe interface to a block cache, wrapping one or
// more block devices.
type BlockCache interface {
	// Attach a new device to the cache
	MountDevice(devno int, dev RandDevice, info DeviceInfo) os.Error
	// Remove a device from the cache
	UnmountDevice(devno int) os.Error
	// Get a block from the cache
	GetBlock(dev, bnum int, btype BlockType, only_search int) *CacheBlock
	// Release (free) a block back to the cache
	PutBlock(cb *CacheBlock, btype BlockType) os.Error
	// Check if the cache contains the given block. If the block exists in the
	// cache, it is marked as ineligible for eviction. If the block is not
	// currently a member of the cache, returns false.
	Reserve(dev, bnum int) bool
	// Claim the reservation on a reserved block. A reservation cannot be
	// revoked, you must Claim and then Put instead.
	Claim(dev, bnum int, btype BlockType) *CacheBlock
	// Invalidate all blocks for a given device
	Invalidate(dev int)
	// Flush any dirty blocks for a given device to the device
	Flush(dev int)
	// Close the block cache
	Close() os.Error
}

type Superblock interface {
	// Allocate a free inode, returning the number of the allocated inode
	AllocInode(mode uint16) (int, os.Error)
	// Allocate a free zone, returning the number of the allocated zone. Start
	// looking at zone number 'zstart' in an attempt to provide contiguous
	// allocation of zones.
	AllocZone(zstart int) (int, os.Error)
	// Free an allocated inode
	FreeInode(inum int)
	// Free an allocated zone
	FreeZone(znum int)
	// Close the superblock
	Close() os.Error
}
