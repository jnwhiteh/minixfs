package common

type StatInfo struct{}

type Inode struct {
	*Disk_Inode // the inode as stored on disk

	Bcache  BlockCache // the block cache for the file system
	Icache  InodeTbl   // the inode table for the file system
	Devinfo DeviceInfo // the device information for this inode's device

	Inum    int    // the inode number of this inode
	Count   int    // the number of clients of this inode
	Dirty   bool   // whether or not this inode has uncommited changes
	Mounted *Inode // the inode that is mounted on top of this one (if any)
}

type DeviceInfo struct {
	MapOffset     int // offset to move past bitmap blocks
	Blocksize     int
	Scale         uint     // Log_zone_scale from the superblock
	Firstdatazone int      // the first data zone on the system
	Zones         int      // the number of zones on the disk
	Inodes        int      // the number of inodes on the dik
	Maxsize       int      // the maximum size of a file on the disk
	ImapBlocks    int      // the number of inode bitmap blocks
	ZmapBlocks    int      // the number of zone bitmap blocks
	Devnum        int      // the number of this decide (if mounted)
	AllocTbl      AllocTbl // the allocation table process
}

type CacheBlock struct {
	Block    Block // the block data structure
	Blocknum int   // the number of this block
	Devnum   int   // the device number of this block
	Dirty    bool  // whether or not the block is dirty

	Buf interface{} // the cache-policy specific block
}

type Fd struct{}

// A interface to a file coupled with position
type Filp interface {
	Seek(pos, whence int) (int, error)
	Read(buf []byte) (int, error)
	Write(buf []byte) (int, error)
	Dup() Filp
	Close() error
}

// Private interface to a file, used by Filp and FileSystem
type File interface {
	Read(buf []byte, pos int) (int, error)
	Write(buf []byte, pos int) (int, error)
	Truncate(length int) error
	Fstat() StatInfo
	Sync() error
	Dup() File
	Close() error
}

type AllocTbl interface {
	AllocInode() (int, error)
	AllocZone(zstart int) (int, error)
	FreeInode(inum int) error
	FreeZone(znum int) error
}

type InodeTbl interface {
	MountDevice(devnum int, info DeviceInfo)
	UnmountDevice(devnum int) error
	GetInode(devnum int, inode int) (*Inode, error)
	DupInode(inode *Inode) *Inode
	PutInode(inode *Inode)
	FlushInode(inode *Inode)
	IsDeviceBusy(devnum int) bool
}

type BlockCache interface {
	MountDevice(devnum int, dev BlockDevice, info DeviceInfo) error
	UnmountDevice(devnum int) error
	GetBlock(devnum, bnum int, btype BlockType, only_search int) *CacheBlock
	PutBlock(cb *CacheBlock, btype BlockType) error
	Invalidate(devnum int)
	Flush(devnum int)
}

type BlockDevice interface {
	Read(buf interface{}, pos int64) error
	Write(buf interface{}, pos int64) error
}
