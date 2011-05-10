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

	procs    []*Process // an array of processes that have been opened
	rootproc *Process   // a 'fake' process providing context for the filesystem
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

	// fetch the root inode
	rip, err := fs.GetInode(ROOT_INODE_NUM)
	if err != nil {
		log.Printf("Unable to fetch root inode: %s", err)
		return nil, err
	}

	fs.rootproc = &Process{fs, 0, 022, rip, rip}
	return fs, nil
}

// Close the filesystem
func (fs *FileSystem) Close() {
	fs.dev.Close()
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

// Skeleton implementation of system calls required for tests in 'fs_test.go'

type Process struct {
	fs      *FileSystem // the file system on which this process resides
	pid     int         // numeric id of the process
	umask   uint16      // file creation mask
	rootdir *Inode      // root directory of the process
	workdir *Inode      // working directory of the process
}

func (proc *Process) Open(path string, flags int, perm int) (*File, os.Error) {
	// TODO Fetch the inode for this file
	var rip *Inode = proc.fs.EatPath(proc, path)
	return &File{proc, 0, rip}, nil
}

var ERR_PID_EXISTS = os.NewError("Process already exists")
var ERR_PATH_LOOKUP = os.NewError("Could not lookup path")

func (fs *FileSystem) NewProcess(pid int, umask uint16, rootpath string) (*Process, os.Error) {
	if fs.procs[pid] != nil {
		return nil, ERR_PID_EXISTS
	}

	// Get an inode from a path
	rip := fs.EatPath(fs.rootproc, rootpath)
	if rip == nil {
		return nil, ERR_PATH_LOOKUP
	}

	rinode := rip
	winode := rinode
	return &Process{fs, pid, umask, rinode, winode}, nil
}

// File represents an open file and is the OO equivalent of the file
// descriptor.
type File struct {
	proc *Process // the process in which this file is opened
	pos  int      // the current position in the file
	rip  *Inode   // the inode for the file
}

// Seek sets the position for the next read or write to pos, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end of the
// file. It returns the new offset and an Error, if any.
//
// TODO: Implement end of file seek and error checking

func (file *File) Seek(pos int, whence int) (int, os.Error) {
	switch whence {
	case 1:
		file.pos += pos
	case 0:
		file.pos = pos
	default:
		panic("NYI: file.Seek with whence > 1")
	}

	return file.pos, nil
}

func (file *File) Read(b []byte) (int, os.Error) {
	// We want to read at most len(b) bytes from the given file. This data
	// will almost certainly be split up amongst multiple blocks.

	// Determine what the ending position to be read is
	endpos := file.pos + len(b)
	if endpos >= int(file.rip.Size) {
		endpos = int(file.rip.Size) - 1
	}

	fs := file.proc.fs

	// We can't just start reading at the start of a block, since we may be at
	// an offset within that block. So work out the first chunk to read
	offset := file.pos % int(fs.Block_size)
	bnum := fs.ReadMap(file.rip, uint(file.pos))

	// TODO: Error check this
	// read the first data block and copy the portion of data we need
	bp := fs.GetBlock(int(bnum), FULL_DATA_BLOCK)
	bdata := bp.block.(FullDataBlock)

	if len(b) < int(fs.Block_size) - offset { // this block contains all the data we need
		for i := 0; i < len(b); i++ {
			b[i] = bdata[offset + i]
		}
		file.pos += len(b)
		fs.PutBlock(bp, FULL_DATA_BLOCK)
		return len(b), nil
	}

	// we need this entire first block, so start filling
	var numBytes int = 0
	for i := 0; i < int(fs.Block_size) - offset; i++ {
		b[i] = bdata[offset + i]
		numBytes++
	}

	fs.PutBlock(bp, FULL_DATA_BLOCK)
	file.pos += numBytes

	// At this stage, all reads should be on block boundaries. The final block
	// will likely be a partial block, so handle that specially.
	for numBytes < len(b) {
		bnum = fs.ReadMap(file.rip, uint(file.pos))
		bp := fs.GetBlock(int(bnum), FULL_DATA_BLOCK)
		bdata := bp.block.(FullDataBlock)

		bytesLeft := len(b) - numBytes // the number of bytes still needed

		// If we only need a portion of this block
		if bytesLeft < int(fs.Block_size) {

			for i := 0; i < bytesLeft; i++ {
				b[numBytes] = bdata[i]
				numBytes++
			}

			file.pos += bytesLeft
			fs.PutBlock(bp, FULL_DATA_BLOCK)
			return numBytes, nil
		}

		// We need this whole block
		for i := 0; i < len(bdata); i++ {
			b[numBytes] = bdata[i]
			numBytes++
		}

		file.pos += len(bdata)
		fs.PutBlock(bp, FULL_DATA_BLOCK)
	}

	return numBytes, nil
}
