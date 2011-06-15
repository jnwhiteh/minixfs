package minixfs

import "encoding/binary"
import "log"
import "math"
import "os"
import "sync"

// FileSystem encapsulates a minix file system, including the shared data
// structures associated with the file system. It abstracts away from the file
// system residing on disk.
type FileSystem struct {
	devs   []BlockDevice // the block devices that comprise the file system
	supers []*Superblock // the superblocks for the given devices

	// These two members are individually locked and protected, although the
	// icache can call into fs.get_block specifically.
	cache  *LRUCache   // the block cache (shared across all devices)
	icache *InodeCache // the inode cache (shared across all devices)

	filp  []*filp    // the filp table
	procs []*Process // an array of processes that have been opened

	m struct { // a struct containing mutexes for the volatile parts of the system
		devs *sync.RWMutex // mutex for both dev/super
		filp *sync.RWMutex
		proc *sync.RWMutex
	}
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

	fs.cache = NewLRUCache()
	fs.icache = NewInodeCache(fs, NR_INODES)

	fs.filp = make([]*filp, NR_FILPS)
	fs.procs = make([]*Process, NR_PROCS)

	fs.m.devs = new(sync.RWMutex)
	fs.m.filp = new(sync.RWMutex)
	fs.m.proc = new(sync.RWMutex)

	fs.devs[ROOT_DEVICE] = dev
	fs.supers[ROOT_DEVICE] = super

	err = fs.cache.MountDevice(ROOT_DEVICE, dev, super)
	if err != nil {
		log.Printf("Could not mount root device: %s", err)
		return nil, err
	}
	err = fs.icache.MountDevice(ROOT_DEVICE, dev, super)
	if err != nil {
		log.Printf("Could not mount root device on icache: %s", err)
		return nil, err
	}

	// fetch the root inode
	rip, err := fs.get_inode(ROOT_DEVICE, ROOT_INODE)
	if err != nil {
		log.Printf("Unable to fetch root inode: %s", err)
		return nil, err
	}

	fs.procs[ROOT_PROCESS] = &Process{fs, 0, 022, rip, rip, make([]*filp, OPEN_MAX)}
	return fs, nil
}

// Close the filesystem
func (fs *FileSystem) Close() {
	fs.m.devs.Lock()
	defer fs.m.devs.Unlock()

	for i := 0; i < NR_SUPERS; i++ {
		if fs.devs[i] != nil {
			fs.cache.Flush(i)
			WriteSuperblock(fs.devs[i], fs.supers[i]) // flush the superblock
			fs.devs[i].Close()
			fs.devs[i] = nil
		}
	}
}

// The get_block method is a wrapper for fs.cache.GetBlock()
func (fs *FileSystem) get_block(dev, bnum int, btype BlockType, only_search int) *buf {
	return fs.cache.GetBlock(dev, bnum, btype, only_search)
}

// The put_block method is a wrapper for fs.cache.PutBlock()
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
	filp    []*filp     // the list of file descriptors
}

var ERR_PID_EXISTS = os.NewError("Process already exists")
var ERR_PATH_LOOKUP = os.NewError("Could not lookup path")

// NewProcess acquires the 'fs.proc' lock in write mode.
func (fs *FileSystem) NewProcess(pid int, umask uint16, rootpath string) (*Process, os.Error) {
	fs.m.proc.Lock()
	defer fs.m.proc.Unlock()

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
	filp := make([]*filp, OPEN_MAX)
	umask = ^umask // convert it so its actually usable as a mask

	return &Process{fs, pid, umask, rinode, winode, filp}, nil
}

var mode_map = []uint16{R_BIT, W_BIT, R_BIT | W_BIT, 0}

// NewProcess acquires the 'fs.filp' lock in write mode.
func (proc *Process) Open(path string, oflags int, omode uint16) (*File, os.Error) {
	proc.fs.m.devs.RLock() // acquire device lock (syscall:open)
	defer proc.fs.m.devs.RUnlock()

	// Remap the bottom two bits of oflags
	bits := mode_map[oflags&O_ACCMODE]

	var err os.Error = nil
	var rip *Inode = nil
	var exist bool = false

	// If O_CREATE is set, try to make the file
	if oflags&O_CREAT > 0 {
		// Create a new node by calling new_node()
		omode := I_REGULAR | (omode & ALL_MODES & proc.umask)
		rip, err = proc.fs.new_node(proc, path, omode, NO_ZONE)
		if err == nil {
			exist = false
		} else if err != EEXIST {
			return nil, err
		} else {
			exist = (oflags&O_EXCL == 0)
		}
	} else {
		// scan path name
		rip, err = proc.fs.eat_path(proc, path)
		if err != nil {
			return nil, err
		}
	}

	// See if a file descriptor and filp slots are available. Unlike the
	// original source, this does allocate a filp slot for us so we need to
	// release it if the open/creat is unsuccessful.
	fd, filpidx, filp, err := proc.fs.get_fd(proc, 0, bits)

	err = nil
	if exist {
		// TODO: Check permissions
		switch rip.GetType() {
		case I_REGULAR:
			if oflags&O_TRUNC > 0 {
				proc.fs.truncate(rip)
				proc.fs.wipe_inode(rip)
				// Send the inode from the inode cache to the block cache, so
				// it gets written on the next cache flush
				proc.fs.icache.WriteInode(rip)
			}
		case I_DIRECTORY:
			// Directories may be read, but not written
			if bits&W_BIT > 0 {
				err = EISDIR
			}
		default:
			// TODO: Add other cases
			panic("NYI: Process.Open with non regular/directory")
		}
	}

	if err != nil {
		// Release the allocated fd
		proc.fs.m.filp.Lock()
		proc.filp[fd] = nil
		proc.fs.filp[filpidx] = nil
		proc.fs.m.filp.Unlock()
	} else {
		// fill in the allocated filp entry
		filp.count = 1
		filp.inode = rip
		filp.pos = 0
		filp.flags = oflags
		filp.mode = bits
	}

	return &File{filp, proc}, nil
}

func (proc *Process) Unlink(path string) os.Error {
	proc.fs.m.devs.RLock() // acquire device lock (syscall:unlink)
	defer proc.fs.m.devs.RUnlock()

	fs := proc.fs
	// Call a helper function to do most of the dirty work for us
	rldirp, rip, rest, err := fs.unlink(proc, path)
	if err != nil || rldirp == nil || rip == nil {
		return err
	}

	// Now test if the call is allowed (altered from Minix)
	if rip.inum == ROOT_INODE {
		err = EBUSY
	}
	if err == nil {
		err = fs.unlink_file(rldirp, rip, rest)
	}

	// If unlink was possible, it has been done, otherwise it has not
	fs.put_inode(rip)
	fs.put_inode(rldirp)
	return err
}


// Perform the mkdir(name, mode) system call
func (proc *Process) Mkdir(path string, mode uint16) os.Error {
	proc.fs.m.devs.RLock() // acquire device lock (syscall:mkdir)
	defer proc.fs.m.devs.RUnlock()

	fs := proc.fs
	var dot, dotdot int
	var err_code os.Error

	// Check to see if it is possible to make another link in the parent
	// directory.
	ldirp, rest, err := fs.last_dir(proc, path) // pointer to new dir's parent
	if ldirp == nil {
		return err
	}
	if ldirp.Nlinks >= math.MaxUint16 {
		fs.put_inode(ldirp)
		return EMLINK
	}

	var rip *Inode

	// Next make the inode. If that fails, return error code
	bits := I_DIRECTORY | (mode & RWX_MODES & proc.umask)
	rip, err_code = fs.new_node(proc, path, bits, 0)
	if rip == nil || err == EEXIST {
		fs.put_inode(rip)   // can't make dir: it already exists
		fs.put_inode(ldirp) // return parent too
		return err_code
	}

	// Get the inode numbers for . and .. to enter into the directory
	dotdot = int(ldirp.inum) // parent's inode number
	dot = int(rip.inum)      // inode number of the new dir itself

	// Now make dir entries for . and .. unless the disk is completely full.
	// Use dot1 and dot2 so the mode of the directory isn't important.
	rip.Mode = bits                                  // set mode
	err1 := fs.search_dir(rip, ".", &dot, ENTER)     // enter . in the new dir
	err2 := fs.search_dir(rip, "..", &dotdot, ENTER) // enter .. in the new dir

	// If both . and .. were successfully entered, increment the link counts
	if err1 == nil && err2 == nil {
		// Normal case. it was possible to enter . and .. in the new dir
		rip.Nlinks++       // this accounts for .
		ldirp.Nlinks++     // this accounts for ..
		ldirp.dirty = true // mark parent's inode as dirty
	} else {
		// It was not possible to enter . and .. or probably the disk was full
		nilinode := 0
		fs.search_dir(ldirp, rest, &nilinode, DELETE) // remove the new directory
		rip.Nlinks--                                  // undo the increment done in new_node
	}

	rip.dirty = true // either way, Nlinks has changed

	fs.put_inode(ldirp)
	fs.put_inode(rip)
	return err_code
}

func (proc *Process) Rmdir(path string) os.Error {
	proc.fs.m.devs.RLock() // acquire device lock (syscall:rmdir)
	defer proc.fs.m.devs.RUnlock()

	fs := proc.fs
	// Call a helper function to do most of the dirty work for us
	rldirp, rip, rest, err := fs.unlink(proc, path)
	if err != nil || rldirp == nil || rip == nil {
		return err
	}

	err = fs.remove_dir(proc, rldirp, rip, rest) // perform the rmdir

	// If unlink was possible, it has been done, otherwise it has not
	fs.put_inode(rip)
	fs.put_inode(rldirp)
	return err
}

func (proc *Process) Chdir(path string) os.Error {
	proc.fs.m.devs.RLock() // acquire device lock (syscall:chdir)
	defer proc.fs.m.devs.RUnlock()

	rip, err := proc.fs.eat_path(proc, path)
	if rip == nil || err != nil {
		return err
	}

	var r os.Error

	if rip.GetType() != I_DIRECTORY {
		r = ENOTDIR
	}
	// TODO: Check permissions

	// If error then return inode
	if r != nil {
		proc.fs.put_inode(rip)
		return r
	}

	// Everything is OK. Make the change.
	proc.fs.put_inode(proc.workdir)
	proc.workdir = rip
	return nil
}

// File represents an open file and is the OO equivalent of the file
// descriptor.
type File struct {
	*filp          // the current position in the file
	proc  *Process // the process in which this file is opened
}

// Seek sets the position for the next read or write to pos, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end of the
// file. It returns the new offset and an Error, if any.
//
// TODO: Implement end of file seek and error checking

func (file *File) Seek(pos int, whence int) (int, os.Error) {
	file.proc.fs.m.devs.RLock() // acquire device lock (syscall)
	defer file.proc.fs.m.devs.RUnlock()

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
	file.proc.fs.m.devs.RLock() // acquire device lock (syscall)
	defer file.proc.fs.m.devs.RUnlock()

	// We want to read at most len(b) bytes from the given file. This data
	// will almost certainly be split up amongst multiple blocks.

	// Determine what the ending position to be read is
	endpos := file.pos + len(b)
	if endpos >= int(file.inode.Size) {
		endpos = int(file.inode.Size) - 1
	}

	fs := file.proc.fs
	dev := file.inode.dev
	blocksize := int(fs.supers[dev].Block_size)

	// We can't just start reading at the start of a block, since we may be at
	// an offset within that block. So work out the first chunk to read
	offset := file.pos % blocksize
	bnum := fs.read_map(file.inode, uint(file.pos))

	// TODO: Error check this
	// read the first data block and copy the portion of data we need
	bp := fs.get_block(dev, int(bnum), FULL_DATA_BLOCK, NORMAL)
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
		bnum = fs.read_map(file.inode, uint(file.pos))
		bp := fs.get_block(dev, int(bnum), FULL_DATA_BLOCK, NORMAL)
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
	file.proc.fs.m.devs.RLock() // acquire device lock (syscall:write)
	defer file.proc.fs.m.devs.RUnlock()

	panic("NYI: File.Write")
}

// TODO: Should this always be succesful?
func (file *File) Close() {
	file.proc.fs.m.devs.RLock() // acquire device lock (syscall:close)
	defer file.proc.fs.m.devs.RUnlock()

	file.proc.fs.put_inode(file.inode)
	file.proc = nil
}
