package common

import (
	"sync"
)

// CacheBlock is a generic 'buffer cache struct that is used throughout the
// file server. We need some metadata in order to make things nicer, so we
// include the block, the block number and the other metadata we need. These
// will be needed for any BlockCache implementation.
type CacheBlock struct {
	Block   Block // the block data structure
	Blockno int   // the number of this block
	Devno   int   // the device number of this block
	Dirty   bool  // whether or not the block is dirty (needs to be written)

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
	Zones         int  // the number of zones on the disk
	Inodes        int  // the number of inodes on the dik
	Maxsize       int  // the maximum size of a file on the disk
	ImapBlocks    int  // the number of inode bitmap blocks
	ZmapBlocks    int  // the number of zone bitmap blocks
}

type Inode struct {
	*Disk_Inode   // the inode as stored on disk
	*sync.RWMutex // this lock must be acquired for any inode operation

	Bcache  BlockCache // the block cache
	Icache  InodeCache // the inode cache
	Bitmap  Bitmap     // the bitmap for the inode's device (for allocation)
	Devinfo DeviceInfo // the device information for the inode's device

	Devnum int // the device number
	Inum   int // the inode number

	Count int  // the number of clients of this inode
	Dirty bool // whether or not the inode has been changed
	Mount bool // whether or not this inode is used as a mount point
}

// The following functions operate on the portions of an inode that cannot
// change except at creation time. They do not need to acquire the mutex in
// order to perform their work, so they are safe.
func (rip *Inode) GetType() int {
	return int(rip.Mode & I_TYPE)
}

func (rip *Inode) IsRegular() bool {
	return rip.Mode&I_TYPE == I_REGULAR
}

func (rip *Inode) IsDirectory() bool {
	return rip.Mode&I_TYPE == I_DIRECTORY
}
