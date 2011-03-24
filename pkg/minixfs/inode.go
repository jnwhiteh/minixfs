package minixfs

// mode_t		uint16
// nlink_t		int16
// uid_t		int16
// gid_t		char
// off_t		int32
// time_t		int32
// zone_t		uint32

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

type Inode struct {
	*disk_inode
	fs    *FileSystem
	count uint
	inum  uint
}

func (inode *Inode) GetType() uint16 {
	return inode.Mode & I_TYPE
}

func (inode *Inode) IsDirectory() bool {
	return inode.GetType() == I_DIRECTORY
}

func (inode *Inode) IsRegular() bool {
	return inode.GetType() == I_REGULAR
}
