package common

type StatInfo struct{}

type MountInfo struct {
	MountPoint  *Inode // the inode on which another file system is mounted
	MountTarget *Inode // the root inode of the mounted file system
}

type Inode struct {
	*Disk_Inode // the inode as stored on disk

	Bcache  BlockCache  // the block cache for the file system
	Icache  InodeTbl    // the inode table for the file system
	Devinfo *DeviceInfo // the device information for this inode's device

	Inum  int  // the inode number of this inode
	Count int  // the number of clients of this inode
	Dirty bool // whether or not this inode has uncommited changes

	Mounted *MountInfo // non-nil if this inode is a mount point or target

	// This field is only present if the inode has been opened as a file for
	// reading and writing.
	File File
}

func (rip *Inode) Type() int {
	return int(rip.Mode & I_TYPE)
}

func (rip *Inode) IsRegular() bool {
	return rip.Mode&I_TYPE == I_REGULAR
}

func (rip *Inode) IsDirectory() bool {
	return rip.Mode&I_TYPE == I_DIRECTORY
}

type DeviceInfo struct {
	MapOffset     int // offset to move past bitmap blocks
	Blocksize     int
	Scale         uint       // Log_zone_scale from the superblock
	Firstdatazone int        // the first data zone on the system
	Zones         int        // the number of zones on the disk
	Inodes        int        // the number of inodes on the dik
	Maxsize       int        // the maximum size of a file on the disk
	ImapBlocks    int        // the number of inode bitmap blocks
	ZmapBlocks    int        // the number of zone bitmap blocks
	Devnum        int        // the number of this decide (if mounted)
	AllocTbl      AllocTbl   // the allocation table process
	MountInfo     *MountInfo // mount point/target for this device
}

type CacheBlock struct {
	Block    Block // the block data structure
	Blocknum int   // the number of this block
	Devnum   int   // the device number of this block
	Dirty    bool  // whether or not the block is dirty

	Buf interface{} // the cache-policy specific block
}

// This is the interface via which a user program will perform file
// operations. These can be performed concurrency.
type Fd interface {
	Seek(pos, whence int) (int, error)
	Read(buf []byte) (int, error)
	Write(buf []byte) (int, error)
	Truncate(length int) error
	Fstat() (*StatInfo, error)
}

// Private interface to a file, used by Filp and FileSystem
type File interface {
	Read(buf []byte, pos int) (int, error)
	Write(buf []byte, pos int) (int, error)
	Truncate(length int) error
	Fstat() (*StatInfo, error)
	Sync() error
	Dup() File
	Close() error
}

type AllocTbl interface {
	AllocInode() (int, error)
	AllocZone(zstart int) (int, error)
	FreeInode(inum int) error
	FreeZone(znum int) error
	Shutdown() error // so the server can be shut down
}

type InodeTbl interface {
	MountDevice(devnum int, info *DeviceInfo)
	UnmountDevice(devnum int) error
	GetInode(devnum int, inode int) (*Inode, error)
	DupInode(inode *Inode) *Inode
	PutInode(inode *Inode)
	FlushInode(inode *Inode)
	IsDeviceBusy(devnum int) bool
	Shutdown() error // so the server can be shut down
}

type BlockCache interface {
	MountDevice(devnum int, dev BlockDevice, info *DeviceInfo) error
	UnmountDevice(devnum int) error
	GetBlock(devnum, bnum int, btype BlockType, only_search int) *CacheBlock
	PutBlock(cb *CacheBlock, btype BlockType) error
	Invalidate(devnum int)
	Flush(devnum int)
	Shutdown() error // so the server can be shut down
}

type BlockDevice interface {
	Read(buf interface{}, pos int64) error
	Write(buf interface{}, pos int64) error
	Close() error
}
