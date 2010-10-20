package minixfs

import "encoding/binary"
import "os"

// This type encapsulates a minix file system, including the shared data
// structures associated with the file system. It abstracts away from the
// file system residing on disk

type FileSystem struct {
	file   *os.File         // the actual file backing the file system
	super  *Superblock      // the superblock for the associated file system
	inodes map[uint]*Inode  // a map containing the inodes for the open files
}

type disk_directory struct {
	Inum uint32
	Name [60]byte
}

// This code needs to be able to handle different types of data blocks, in
// particular ordinary user data, directory blocks, indirect blocks, inode
// blocks, and bitmap blocks

type InodeBlock_16 struct {
	Data [16]disk_inode
}

type DirectoryBlock_16 struct {
	Data [16]disk_directory
}

// Create a new FileSystem from a given file on the filesystem
func OpenFileSystemFile(filename string) (*FileSystem, os.Error) {
	var fs *FileSystem = new(FileSystem)

	// open the file, but do not close it
	file, err := os.Open(filename,os. O_RDWR, 0)

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

	return fs, nil
}

func (fs *FileSystem) GetMagic() (uint16) {
	return fs.super.Magic
}

func (fs *FileSystem) GetInode(num uint) (*Inode, os.Error) {
	if num == 0 {
		return nil, os.NewError("Attempt to get invalid inode '0'")
	}

	// Check and see if the inode is already loaded in memory
	if inode, ok := fs.inodes[num]; ok {
		inode.count++
		return inode, nil
	}

	if len(fs.inodes) >= NR_INODES {
		return nil, os.NewError("Too many open inodes")
	}

	// Load the inode from the disk and create in-memory version of it
	offset := fs.super.Imap_blocks + fs.super.Zmap_blocks + 2
	blockNum := ((num - 1) / fs.super.inodes_per_block) + uint(offset)
	println("offset: ", offset)
	println("blocknum: ", blockNum)

	inode_block := new(InodeBlock_16)

	err := fs.GetBlock(blockNum, inode_block)
	if err != nil {
		return nil, err
	}

	// We have the full block, now get the correct inode entry 
	inode_d := &inode_block.Data[num % fs.super.inodes_per_block]
	inode := &Inode{inode_d, fs, 1, num}

	return inode, nil
}

func (fs *FileSystem) GetDataBlockFromZone(num uint) (uint) {
	// Move past the boot block, superblock and bitmats
	offset := uint(2 + fs.super.Imap_blocks + fs.super.Zmap_blocks)
	offset = offset + (uint(fs.super.Ninodes) / fs.super.inodes_per_block)
	return offset + num
}

func (fs *FileSystem) GetBlock(num uint, block interface{}) (os.Error) {
	if num <= 1 {
		panic("TODO: Fix this")
	}

	// Adjust the file position according to two static blocks at start
	pos := int64((num) * uint(fs.super.Block_size))
	println("seeking to pos: ", pos)
	newPos, err := fs.file.Seek(pos, 0)
	if err != nil || pos != newPos {
		return err
	}

	err = binary.Read(fs.file, binary.LittleEndian, block)
	if err != nil {
		return err
	}

	return nil
}
