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

type InodeCache interface {
	// Update the information about a given device
	Mount(devno int, super Superblock, info DeviceInfo)
	// Get an inode from the given device
	GetInode(devno, inum int) (*CacheInode, os.Error)
	// Return the given inode to the cache. If the inode has been altered and
	// it has no other clients, it should be written to the block cache.
	PutInode(rip *CacheInode)
	// Returns whether or not the given device is busy. As non-busy device has
	// exactly one client of the root inode.
	IsDeviceBusy(devno int) bool
	// Close the inode cache
	Close() os.Error
}
