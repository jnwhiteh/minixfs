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

// CacheInode is a self-aware wrapper around an inode stored on disk.
type CacheInode struct {
	Inode   *Disk_Inode // the inode as stored on disk
	Bitmap  Bitmap
	Devinfo DeviceInfo
	Devno   int
	Inum    int
	Count   int
	Dirty   bool
	Mount   bool
	Server  interface{}
}

func (ci *CacheInode) GetType() int {
	return int(ci.Inode.Mode & I_TYPE)
}

func (ci *CacheInode) IsRegular() bool {
	if ci.Inode == nil {
		return false
	}
	return ci.Inode.Mode&I_TYPE == I_REGULAR
}

func (ci *CacheInode) IsDirectory() bool {
	if ci.Inode == nil {
		return false
	}
	return ci.Inode.Mode&I_TYPE == I_DIRECTORY
}

func (ci *CacheInode) Dinode() Dinode {
	if ci.Inode == nil {
		return nil
	} else if !ci.IsDirectory() {
		return nil
	}

	if dinode, ok := ci.Server.(Dinode); ok {
		return dinode
	}
	return nil
}

func (ci *CacheInode) Finode() Finode {
	if ci.Inode == nil {
		return nil
	} else if !ci.IsRegular() {
		return nil
	}

	if finode, ok := ci.Server.(Finode); ok {
		return finode
	}
	return nil
}
