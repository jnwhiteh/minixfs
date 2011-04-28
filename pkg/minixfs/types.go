package minixfs

import "unsafe"

type bit_t uint32
type bitchunk_t uint16
type gid_t byte
type ino_t uint32
type mode_t uint16
type nlink_t int16
type off_t int32
type time_t int32
type uid_t int16
type zone_t uint32
type zone1_t uint16

const (
	Sizeof_bitchunk_t = uint(unsafe.Sizeof(*new(bitchunk_t)))
)

type disk_superblock struct {
	Ninodes           uint32 // # of usable inodes on the minor device
	Nzones            uint16 // total device size, including bit maps, etc.
	Imap_blocks       uint16 // # of blocks used by inode bit map
	Zmap_blocks       uint16 // # of blocks used by zone bit map
	Firstdatazone_old uint16 // number of first data zone
	Log_zone_size     uint16 // log2 of blocks/zone
	Pad               uint16 // try to avoid compiler-dependent padding
	Max_size          int32  // maximum file size on this device
	Zones             uint32 // number of zones (replaces s_nzones in V2+)
	Magic             uint16 // magic number to recognize super-blocks

	// The following fields are only present in V3 and above, which is
	// the standard version of Minix that we are implementing
	Pad2         uint16 // try to avoid compiler-dependent padding
	Block_size   uint16 // block size in bytes
	Disk_version byte   // filesystem format sub-version
}

type disk_inode struct {
	Mode   uint16 // file type, protection, etc.
	Nlinks uint16 // how many links to this file. HACK!
	Uid    int16  // user id of the file's owner
	Gid    uint16 // group number. HACK!
	Size   int32  // current file size in bytes
	Atime  int32  // when was file data last accessed
	Mtime  int32  // when was file data last changed
	Ctime  int32  // when was inode data last changed
	Zone   [10]uint32
}

type disk_dirent struct {
	Inum uint32
	Name [60]byte
}
