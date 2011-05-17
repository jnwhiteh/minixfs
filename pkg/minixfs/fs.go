package minixfs

import "encoding/binary"
import "log"
import "os"
import "sync"

// FileSystem encapsulates a minix file system, including the shared data
// structures associated with the file system. It abstracts away from the file
// system residing on disk.
type FileSystem struct {
	devs   []BlockDevice // the block devices that comprise the file system
	supers []*Superblock // the superblocks for the given devices
	inodes []*Inode      // the block of in-core inode entries
	cache  *LRUCache     // the block cache (shared across all devices)

	procs []*Process // an array of processes that have been opened

	mutex *sync.RWMutex // mutex for reading/writing the above arrays
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

	fs.devs = make([]BlockDevice, NR_SUPERS)
	fs.supers = make([]*Superblock, NR_SUPERS)
	fs.inodes = make([]*Inode, NR_INODES)
	fs.procs = make([]*Process, NR_PROCS)

	fs.cache = NewLRUCache(fs, NR_BUFS)
	fs.mutex = new(sync.RWMutex)

	fs.devs[ROOT_DEVICE] = dev
	fs.supers[ROOT_DEVICE] = super

	// fetch the root inode
	rip, err := fs.get_inode(ROOT_DEVICE, ROOT_INODE)
	if err != nil {
		log.Printf("Unable to fetch root inode: %s", err)
		return nil, err
	}

	fs.procs[ROOT_PROCESS] = &Process{fs, 0, 022, rip, rip}
	return fs, nil
}

// Close the filesystem
func (fs *FileSystem) Close() {
	fs.mutex.Lock()
	for i := 0; i < NR_SUPERS; i++ {
		if fs.devs[i] != nil {
			fs.cache.Flush(i)
			fs.devs[i].Close()
			fs.devs[i] = nil
		}
	}
	fs.mutex.Unlock()
}

// The GetBlock method is a wrapper for fs.cache.GetBlock()
func (fs *FileSystem) get_block(dev, bnum int, btype BlockType) *buf {
	return fs.cache.GetBlock(dev, bnum, btype, false)
}

// The PutBlock method is a wrapper for fs.cache.PutBlock()
func (fs *FileSystem) put_block(bp *buf, btype BlockType) {
	fs.cache.put_block(bp, btype)
}

// Skeleton implementation of system calls required for tests in 'fs_test.go'
type Process struct {
	fs      *FileSystem // the file system on which this process resides
	pid     int         // numeric id of the process
	umask   uint16      // file creation mask
	rootdir *Inode      // root directory of the process
	workdir *Inode      // working directory of the process
}

var ERR_PID_EXISTS = os.NewError("Process already exists")
var ERR_PATH_LOOKUP = os.NewError("Could not lookup path")

func (fs *FileSystem) NewProcess(pid int, umask uint16, rootpath string) (*Process, os.Error) {
	if fs.procs[pid] != nil {
		return nil, ERR_PID_EXISTS
	}

	// Get an inode from a path
	rip, err := fs.eat_path(fs.procs[ROOT_PROCESS], rootpath)
	if err != nil {
		return nil, err
	}

	rinode := rip
	winode := rinode
	return &Process{fs, pid, umask, rinode, winode}, nil
}

func (proc *Process) Open(path string, flags int, perm int) (*File, os.Error) {
	rip, err := proc.fs.eat_path(proc, path)
	if err != nil {
		return nil, err
	}
	return &File{proc, 0, rip}, nil
}

func (proc *Process) Unlink(path string) os.Error {
	panic("NYI: Process.Unlink")
}

func (proc *Process) Mkdir(path string, mode mode_t) os.Error {
	panic("NYI: Process.Mkdir")
}

func (proc *Process) Chdir(path string) os.Error {
	panic("NYI: Process.Chdir")
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
	dev := file.rip.dev
	blocksize := int(fs.supers[dev].Block_size)

	// We can't just start reading at the start of a block, since we may be at
	// an offset within that block. So work out the first chunk to read
	offset := file.pos % blocksize
	bnum := fs.read_map(file.rip, uint(file.pos))

	// TODO: Error check this
	// read the first data block and copy the portion of data we need
	bp := fs.get_block(dev, int(bnum), FULL_DATA_BLOCK)
	bdata := bp.block.(FullDataBlock)

	if len(b) < blocksize-offset { // this block contains all the data we need
		for i := 0; i < len(b); i++ {
			b[i] = bdata[offset+i]
		}
		file.pos += len(b)
		fs.put_block(bp, FULL_DATA_BLOCK)
		return len(b), nil
	}

	// we need this entire first block, so start filling
	var numBytes int = 0
	for i := 0; i < blocksize-offset; i++ {
		b[i] = bdata[offset+i]
		numBytes++
	}

	fs.put_block(bp, FULL_DATA_BLOCK)
	file.pos += numBytes

	// At this stage, all reads should be on block boundaries. The final block
	// will likely be a partial block, so handle that specially.
	for numBytes < len(b) {
		bnum = fs.read_map(file.rip, uint(file.pos))
		bp := fs.get_block(dev, int(bnum), FULL_DATA_BLOCK)
		bdata := bp.block.(FullDataBlock)

		bytesLeft := len(b) - numBytes // the number of bytes still needed

		// If we only need a portion of this block
		if bytesLeft < blocksize {

			for i := 0; i < bytesLeft; i++ {
				b[numBytes] = bdata[i]
				numBytes++
			}

			file.pos += bytesLeft
			fs.put_block(bp, FULL_DATA_BLOCK)
			return numBytes, nil
		}

		// We need this whole block
		for i := 0; i < len(bdata); i++ {
			b[numBytes] = bdata[i]
			numBytes++
		}

		file.pos += len(bdata)
		fs.put_block(bp, FULL_DATA_BLOCK)
	}

	return numBytes, nil
}

func (file *File) Write(data []byte) (n int, err os.Error) {
	panic("NYI: File.Write")
}
