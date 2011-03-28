package minixfs

import "log"

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

// Allocate a free inode on the given FileSystem and return a pointer to it.
func (fs *FileSystem) AllocInode(mode uint16) *Inode {
	// Acquire an inode from the bit map
	b := fs.AllocBit(IMAP, fs.super.I_Search)
	if b == NO_BIT {
		log.Printf("Out of i-nodes on device")
		return nil
	}

	fs.super.I_Search = b

	// Try to acquire a slot in the inode table
	inode, err := fs.GetInode(b)
	if err != nil {
		log.Printf("Failed to get inode: %d", b)
		return nil
	}

	inode.Mode = mode
	inode.Nlinks = 0
	inode.Uid = 0 // TODO: Must get the current uid
	inode.Gid = 0 // TODO: Must get the current gid

	fs.WipeInode(inode)
	return inode
}

func (fs *FileSystem) WipeInode(inode *Inode) {
	inode.Size = 0
	// TODO: Update ATIME, CTIME, MTIME
	// TODO: Make this dirty so its written back out
	inode.Zone = *new([10]uint32)
	for i := 0; i < 10; i++ {
		inode.Zone[i] = NO_ZONE
	}
}
