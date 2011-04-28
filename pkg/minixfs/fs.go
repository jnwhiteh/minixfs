package minixfs

import "log"
import "os"

// This type encapsulates a minix file system, including the shared data
// structures associated with the file system. It abstracts away from the
// file system residing on disk.

type FileSystem struct {
	file   *os.File        // the actual file backing the file system
	super  *Superblock     // the superblock for the associated file system
	inodes map[uint]*Inode // a map containing the inodes for the open files

	Magic         uint // magic number to recognize super-blocks
	Block_size    uint // block size in bytes
	Log_zone_size uint // log2 of blocks/zone

	// TODO: These should be contained in a process table, not in the FileSystem
	RootDir *Inode
	WorkDir *Inode
}

// Create a new FileSystem from a given file on the filesystem
func OpenFileSystemFile(filename string) (*FileSystem, os.Error) {
	var fs *FileSystem = new(FileSystem)

	// open the file, but do not close it
	file, err := os.OpenFile(filename, os.O_RDWR, 0)

	if err != nil {
		return nil, err
	}

	super, err := ReadSuperblock(file)
	if err != nil {
		return nil, err
	}

	fs.file = file
	fs.super = super
	fs.inodes = make(map[uint]*Inode)

	fs.Magic = super.Magic
	fs.Block_size = super.Block_size
	fs.Log_zone_size = super.Log_zone_size

	fs.RootDir, err = fs.GetInode(ROOT_INODE_NUM)
	if err != nil {
		log.Printf("Unable to fetch root inode: %s", err)
		return nil, err
	}

	fs.WorkDir = fs.RootDir

	return fs, nil
}

func (fs *FileSystem) GetDataBlockFromZone(num uint) uint {
	// Move past the boot block, superblock and bitmats
	offset := uint(2 + fs.super.Imap_blocks + fs.super.Zmap_blocks)
	offset = offset + (uint(fs.super.Ninodes) / fs.super.inodes_per_block)
	return offset + num
}
