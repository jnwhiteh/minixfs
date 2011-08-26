package minixfs

import (
	"os"
)

// The buffer cache in the Minix file system is built on top of the raw device
// and provides access to recently requested blocks served from memory.
//
// When a block is obtained via 'GetBlock' its count field is incremented and
// marks it in-use. When a block is returned using 'PutBlock', this same field
// is decremented. Only if this field is 0 is the block actually moved to the
// list of blocks available for eviction.

type BlockType int

const (
	INODE_BLOCK        BlockType = 0 // inode block
	DIRECTORY_BLOCK    BlockType = 1 // directory block
	INDIRECT_BLOCK     BlockType = 2 // pointer block
	MAP_BLOCK          BlockType = 3 // bit map
	FULL_DATA_BLOCK    BlockType = 5 // data, fully used
	PARTIAL_DATA_BLOCK BlockType = 6 // data, partly used
)

// BlockCache is a thread-safe interface to a block cache, wrapping one or
// more devices.
type BlockCache interface {
	// Attach a new device to the cache
	MountDevice(devno int, dev BlockDevice, super *Superblock) os.Error
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

// Buf is a generic 'buffer cache struct that is used throughout the file
// server. We need some metadata in order to make things nicer, so we include
// the block, the block number and the other metadata we need. These will be
// needed for any BlockCache implementation.
type CacheBlock struct {
	block   Block // the block data structure
	blocknr int   // the number of this block
	dev     int   // the device number of this block
	dirty   bool  // whether or not the block is dirty (needs to be written)
	count   int   // the number of users of this block

	// This is a single pointer to a higher-level buf structure, so the cache
	// policy can correlate a given CacheBlock easily with the correct cache
	// entry.
	buf interface{}
}
