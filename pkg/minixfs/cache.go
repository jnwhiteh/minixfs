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
}

type BlockCache interface {
	MountDevice(devno int, dev BlockDevice, super *Superblock) os.Error
	UnmountDevice(devno int) os.Error
	GetBlock(dev, bnum int, btype BlockType, only_search int) *CacheBlock
	PutBlock(cb *CacheBlock, btype BlockType) os.Error
	Invalidate(dev int)
	Flush(dev int)
}
