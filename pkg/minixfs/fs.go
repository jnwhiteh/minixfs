package minixfs

import "encoding/binary"
import "log"
import "os"

// This type encapsulates a minix file system, including the shared data
// structures associated with the file system. It abstracts away from the
// file system residing on disk.

type FileSystem struct {
	dev    BlockDevice     // the underlying filesystem device
	super  *Superblock     // the superblock for the associated file system
	cache  *LRUCache       // the block cache
	inodes map[uint]*Inode // a map containing the inodes for the open files

	Magic         uint // magic number to recognize super-blocks
	Block_size    uint // block size in bytes
	Log_zone_size uint // log2 of blocks/zone

	procs []*Process // an array of processes that have been opened
	RootDir *Inode
	WorkDir *Inode
}

// Create a new FileSystem from a given file on the filesystem
func OpenFileSystemFile(filename string) (*FileSystem, os.Error) {
	var fs *FileSystem = new(FileSystem)

	dev, err := NewFileDevice(filename, binary.LittleEndian)

	if err != nil {
		return nil, err
	}

	super, err := ReadSuperblock(dev)
	if err != nil {
		return nil, err
	}

	fs.dev = dev
	fs.super = super
	fs.cache = NewLRUCache(dev, int(super.Block_size), DEFAULT_NR_BUFS)
	fs.inodes = make(map[uint]*Inode)

	fs.Magic = super.Magic
	fs.Block_size = super.Block_size
	fs.Log_zone_size = super.Log_zone_size

	fs.procs = make([]*Process, NR_PROCS)

	fs.RootDir, err = fs.GetInode(ROOT_INODE_NUM)
	if err != nil {
		log.Printf("Unable to fetch root inode: %s", err)
		return nil, err
	}

	fs.WorkDir = fs.RootDir

	return fs, nil
}

// The GetBlock method is a wrapper for fs.cache.GetBlock()
func (fs *FileSystem) GetBlock(bnum int, btype BlockType) *buf {
	return fs.cache.GetBlock(bnum, btype, false)
}

// The PutBlock method is a wrapper for fs.cache.PutBlock()
func (fs *FileSystem) PutBlock(bp *buf, btype BlockType) {
	fs.cache.PutBlock(bp, btype)
}

func (fs *FileSystem) GetDataBlockFromZone(num uint) uint {
	// Move past the boot block, superblock and bitmats
	offset := uint(2 + fs.super.Imap_blocks + fs.super.Zmap_blocks)
	offset = offset + (uint(fs.super.Ninodes) / fs.super.inodes_per_block)
	return offset + num
}

type Process struct {
	pid     int    // numeric id of the process
	umask   uint16 // file creation mask
	rootdir *Inode // root directory of the process
	workdir *Inode // working directory of the process
}

func (p *Process) Open(path string, flags int, perm int) (*File, os.Error) {
	return &File{}, nil
}

var ERR_PID_EXISTS = os.NewError("Process already exists")

func (fs *FileSystem) NewProcess(pid int, umask uint16, rootpath string) (*Process, os.Error) {
	if fs.procs[pid] != nil {
		return nil, ERR_PID_EXISTS
	}

	// Get an inode from a path
	rip, err := fs.EatPath(rootpath)
	if err != nil || rip == nil {
		return nil, err
	}

	rinode := rip
	winode := rinode
	return &Process{pid, umask, rinode, winode}, nil
}

type File struct {

}

func (f *File) Seek(pos int, whence int) (int, os.Error) {
	return 0, nil
}

func (f *File) Read(b []byte) (int, os.Error) {
	return 0, nil
}
