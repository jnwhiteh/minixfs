package common

// CacheBlock is a generic 'buffer cache struct that is used throughout the
// file server. We need some metadata in order to make things nicer, so we
// include the block, the block number and the other metadata we need. These
// will be needed for any BlockCache implementation.
type CacheBlock struct {
	Block   Block // the block data structure
	Blockno int   // the number of this block
	Devno   int   // the device number of this block
	Dirty   bool  // whether or not the block is dirty (needs to be written)
	Count   int   // the number of users of this block

	// This is a single pointer to a higher-level buf structure, so the cache
	// policy can correlate a given CacheBlock easily with the correct cache
	// entry.
	Buf interface{}
}

type DeviceInfo struct {
	MapOffset     int // offset to move past bitmap blocks
	Blocksize     int
	Scale         uint // Log_zone_scale from the superblock
	Firstdatazone int  // the first data zone on the system
	Zones         int
	Maxsize       int
}

// CacheInode is a self-aware wrapper around an inode stored on disk.
type CacheInode struct {
	Inode *Disk_Inode // the inode as stored on disk
	Devno int
	Inum  int
	Count int
	Dirty bool
	Mount bool
}
