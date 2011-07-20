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

	_filp  []*filp    // the filp table
	_procs []*Process // an array of processes that have been opened

	m struct {
		// A device lock to be used at the system-call level. All system calls
		// must be performed under this mutex, with any system calls that
		// alter the device table (Mount, Unmount and Close) holding the write
		// lock as well as the read lock.
		device *sync.RWMutex
		procs  *sync.RWMutex
		filp   *sync.RWMutex
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

	fs._filp = make([]*filp, NR_FILPS)
	fs._procs = make([]*Process, NR_PROCS)

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

	fs._procs[ROOT_PROCESS] = &Process{fs, 0, 022, rip, rip,
		make([]*filp, OPEN_MAX),
		make([]*File, OPEN_MAX),
		new(sync.RWMutex)}

	fs.m.device = new(sync.RWMutex)
	fs.m.procs = new(sync.RWMutex)
	fs.m.filp = new(sync.RWMutex)

	return fs, nil
}

// Close the filesystem
func (fs *FileSystem) Close() {
	fs.m.device.Lock()
	defer fs.m.device.Unlock()

	for i := 0; i < NR_SUPERS; i++ {
		if fs.devs[i] != nil {
			fs.cache.Flush(i)
			WriteSuperblock(fs.devs[i], fs.supers[i]) // flush the superblock
			fs.devs[i].Close()
			fs.devs[i] = nil
		}
	}
// Mount the filesystem on 'dev' at 'path' in the root filesystem
func (fs *FileSystem) Mount(dev BlockDevice, path string) os.Error {
	fs.m.device.Lock()
	defer fs.m.device.Unlock()

	return fs.do_mount(dev, path)
}

// Unmount a file system by device
func (fs *FileSystem) Unmount(dev BlockDevice) os.Error {
	fs.m.device.Lock()
	defer fs.m.device.Unlock()

	return fs.do_unmount(dev)
}

// The get_block method is a wrapper for fs.cache.GetBlock()
func (fs *FileSystem) get_block(dev, bnum int, btype BlockType, only_search int) *buf {
	return fs.cache.GetBlock(dev, bnum, btype, only_search)
}

// The put_block method is a wrapper for fs.cache.PutBlock()
func (fs *FileSystem) put_block(bp *buf, btype BlockType) {
	fs.cache.PutBlock(bp, btype)
}

// Skeleton implementation of system calls required for tests in 'fs_test.go'
type Process struct {
	fs      *FileSystem // the file system on which this process resides
	pid     int         // numeric id of the process
	umask   uint16      // file creation mask
	rootdir *Inode      // root directory of the process
	workdir *Inode      // working directory of the process
	_filp   []*filp     // the list of file descriptors
	_files  []*File     // the list of open files

	m_filp *sync.RWMutex
}

var ERR_PID_EXISTS = os.NewError("Process already exists")
var ERR_PATH_LOOKUP = os.NewError("Could not lookup path")

func (fs *FileSystem) NewProcess(pid int, umask uint16, rootpath string) (*Process, os.Error) {
	fs.m.device.RLock()
	defer fs.m.device.RUnlock()

	fs.m.procs.Lock()
	defer fs.m.procs.Unlock()

	if fs._procs[pid] != nil {
		return nil, ERR_PID_EXISTS
	}

	// Get an inode from a path
	rip, err := fs.eat_path(fs._procs[ROOT_PROCESS], rootpath)
	if err != nil {
		return nil, err
	}

	rinode := rip
	winode := rinode
	filp := make([]*filp, OPEN_MAX)
	files := make([]*File, OPEN_MAX)
	umask = ^umask // convert it so its actually usable as a mask

	proc := &Process{fs, pid, umask, rinode, winode, filp, files, new(sync.RWMutex)}
	fs._procs[pid] = proc
	return proc, nil
}

func (proc *Process) Exit() {
	proc.fs.m.device.RLock()
	defer proc.fs.m.device.RUnlock()

	fs := proc.fs

	// For each file that is open, close it
	proc.m_filp.Lock()
	for i := 0; i < len(proc._files); i++ {
		if proc._files[i] != nil {
			file := proc._files[i]
			file.close()
		}
	}
	proc.m_filp.Unlock()

	fs.m.procs.Lock()
	fs._procs[proc.pid] = nil
	fs.m.procs.Unlock()

	if proc.workdir != proc.rootdir {
		fs.put_inode(proc.workdir)
	}
	fs.put_inode(proc.rootdir)
}

var mode_map = []uint16{R_BIT, W_BIT, R_BIT | W_BIT, 0}

// NewProcess acquires the 'fs.filp' lock in write mode.
func (proc *Process) Open(path string, oflags int, omode uint16) (*File, os.Error) {
	proc.fs.m.device.RLock()
	defer proc.fs.m.device.RUnlock()

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

	// Allocate a file descriptor and filp slot. This function will put a
	// static 'inUse' filp entry into both the fs/proc tables to prevent
	// re-allocation of the slot returned. As a result, if the open/creat
	// fails, this allocation needs to be reversed.
	fd, filpidx, err := proc.fs.reserve_fd(proc, 0, bits)
	var filp *filp

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

	// We need to alter the filp table here, so grab the mutex again
	proc.fs.m.filp.Lock()
	defer proc.fs.m.filp.Unlock()

	if err != nil {
		// Something went wrong, release the filp reservation
		proc._filp[fd] = nil
		proc.fs._filp[filpidx] = nil

		return nil, err
	} else {
		// Allocate a proper filp entry and update fs/filp tables
		filp = NewFilp(bits, oflags, rip, 1, 0)
		proc._filp[fd] = filp
		proc.fs._filp[filpidx] = filp
	}

	file := &File{filp, proc, fd}
	proc._files[fd] = file
	return file, nil
}

func (proc *Process) Unlink(path string) os.Error {
	proc.fs.m.device.RLock()
	defer proc.fs.m.device.RUnlock()

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
	proc.fs.m.device.RLock()
	defer proc.fs.m.device.RUnlock()

	fs := proc.fs
	var dot, dotdot int
	var err_code os.Error

	// Check to see if it is possible to make another link in the parent
	// directory.
	ldirp, rest, err := fs.last_dir(proc, path) // pointer to new dir's parent
	if ldirp == nil {
		return err
	}
	if ldirp.Nlinks() >= math.MaxUint16 {
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
	rip.SetMode(bits)                                // set mode
	err1 := fs.search_dir(rip, ".", &dot, ENTER)     // enter . in the new dir
	err2 := fs.search_dir(rip, "..", &dotdot, ENTER) // enter .. in the new dir

	// If both . and .. were successfully entered, increment the link counts
	if err1 == nil && err2 == nil {
		// Normal case. it was possible to enter . and .. in the new dir
		rip.IncNlinks()      // this accounts for .
		ldirp.IncNlinks()    // this accounts for ..
		ldirp.SetDirty(true) // mark parent's inode as dirty
	} else {
		// It was not possible to enter . and .. or probably the disk was full
		nilinode := 0
		fs.search_dir(ldirp, rest, &nilinode, DELETE) // remove the new directory
		rip.DecNlinks()                               // undo the increment done in new_node
	}

	rip.SetDirty(true) // either way, Nlinks has changed

	fs.put_inode(ldirp)
	fs.put_inode(rip)
	return err_code
}

func (proc *Process) Rmdir(path string) os.Error {
	proc.fs.m.device.RLock()
	defer proc.fs.m.device.RUnlock()

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
	proc.fs.m.device.RLock()
	defer proc.fs.m.device.RUnlock()

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
	fd    int      // the numeric file descriptor in the process for this file
}

// Seek sets the position for the next read or write to pos, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end of the
// file. It returns the new offset and an Error, if any.
//
// TODO: Implement end of file seek and error checking

func (file *File) Seek(pos int, whence int) (int, os.Error) {
	if file.fd == NO_FILE {
		return 0, EBADF
	}

	file.proc.fs.m.device.RLock()
	defer file.proc.fs.m.device.RUnlock()

	switch whence {
	case 1:
		file.SetPosDelta(pos)
	case 0:
		file.SetPos(pos)
	default:
		panic("NYI: file.Seek with whence > 1")
	}

	return file.Pos(), nil
}

// Read up to len(b) bytes from 'file' from the current position within the
// file.
func (file *File) Read(b []byte) (int, os.Error) {
	if file.fd == NO_FILE {
		return 0, EBADF
	}

	file.proc.fs.m.device.RLock()
	defer file.proc.fs.m.device.RUnlock()

	// We want to read at most len(b) bytes from the given file. This data
	// will almost certainly be split up amongst multiple blocks.
	curpos := file.Pos()

	// Determine what the ending position to be read is
	endpos := curpos + len(b)
	fsize := int(file.inode.Size())
	if endpos >= int(fsize) {
		endpos = int(fsize) - 1
	}

	fs := file.proc.fs
	dev := file.inode.dev
	blocksize := int(fs.supers[dev].Block_size)

	// We can't just start reading at the start of a block, since we may be at
	// an offset within that block. So work out the first chunk to read
	offset := curpos % blocksize
	bnum := fs.read_map(file.inode, uint(curpos))

	// TODO: Error check this
	// read the first data block and copy the portion of data we need
	bp := fs.get_block(dev, int(bnum), FULL_DATA_BLOCK, NORMAL)
	bdata, bok := bp.block.(FullDataBlock)
	if !bok {
		// TODO: Attempt to read from an invalid location, what should happen?
		return 0, EINVAL
	}

	if len(b) < blocksize-offset { // this block contains all the data we need
		for i := 0; i < len(b); i++ {
			b[i] = bdata[offset+i]
		}
		curpos += len(b)
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
	curpos += numBytes

	// At this stage, all reads should be on block boundaries. The final block
	// will likely be a partial block, so handle that specially.
	for numBytes < len(b) {
		bnum = fs.read_map(file.inode, uint(curpos))
		bp := fs.get_block(dev, int(bnum), FULL_DATA_BLOCK, NORMAL)
		if _, sok := bp.block.(FullDataBlock); !sok {
			log.Printf("block num: %d, count: %d", bp.blocknr, bp.count)
			log.Panicf("When reading block %d for position %d, got IndirectBlock", bnum, curpos)
		}

		bdata = bp.block.(FullDataBlock)

		bytesLeft := len(b) - numBytes // the number of bytes still needed

		// If we only need a portion of this block
		if bytesLeft < blocksize {

			for i := 0; i < bytesLeft; i++ {
				b[numBytes] = bdata[i]
				numBytes++
			}

			curpos += bytesLeft
			fs.put_block(bp, FULL_DATA_BLOCK)
			return numBytes, nil
		}

		// We need this whole block
		for i := 0; i < len(bdata); i++ {
			b[numBytes] = bdata[i]
			numBytes++
		}

		curpos += len(bdata)
		fs.put_block(bp, FULL_DATA_BLOCK)
	}

	// TODO: Update this as we read block after block?
	file.SetPos(curpos)

	return numBytes, nil
}

// Write a slice of bytes to the file at the current position. Returns the
// number of bytes actually written and an error (if any).
func (file *File) Write(data []byte) (n int, err os.Error) {
	if file.fd == NO_FILE {
		return 0, EBADF
	}

	file.proc.fs.m.device.RLock()
	defer file.proc.fs.m.device.RUnlock()

	// TODO: This implementation is direct and doesn't match the abstractions
	// in the original source. At some point it should be reviewed.
	cum_io := 0
	position := int(file.Pos())
	fsize := int(file.inode.Size())

	fs := file.proc.fs
	super := fs.supers[file.inode.dev]
	// Check in advance to see if file will grow too big
	if position > (int(super.Max_size) - len(data)) {
		return 0, EFBIG
	}

	// Check for O_APPEND flag
	if file.flags&O_APPEND > 0 {
		position = fsize
	}

	// Clear the zone containing the current present EOF if hole about to be
	// created. This is necessary because all unwritten blocks prior to the
	// EOF must read as zeros.
	if position > fsize {
		fs.clear_zone(file.inode, uint(fsize), 0)
	}

	bsize := int(super.Block_size)
	nbytes := len(data)
	// Split the transfer into chunks that don't span two blocks.
	for nbytes != 0 {
		off := (position % bsize)
		chunk := _MIN(nbytes, bsize-off)
		if chunk < 0 {
			chunk = bsize - off
		}

		// Read or write 'chunk' bytes, fetch the first block
		err = fs.write_chunk(file.inode, position, off, chunk, data)
		if err != nil {
			break // EOF reached
		}

		// Update counters and pointers
		data = data[chunk:] // user buffer
		nbytes -= chunk     // bytes yet to be written
		cum_io += chunk     // bytes written so far
		position += chunk   // position within the file
	}

	if file.inode.GetType() == I_REGULAR || file.inode.GetType() == I_DIRECTORY {
		if position > fsize {
			file.inode.SetSize(int32(position))
		}
	}

	file.SetPos(position)

	// TODO: Update times
	if err == nil {
		file.inode.SetDirty(true)
	}

	return cum_io, err
}

// A non-locking version of the close logic, to be called from proc.Exit and
// file.Close().
func (file *File) close() {
	file.proc.fs.put_inode(file.inode)

	proc := file.proc
	proc._filp[file.fd] = nil
	proc._files[file.fd] = nil

	file.filp.SetCountDelta(-1)
	file.proc = nil
	file.fd = NO_FILE
}

// TODO: Should this always be succesful?
func (file *File) Close() os.Error {
	if file.fd == NO_FILE {
		return EBADF
	}

	file.proc.fs.m.device.RLock()
	defer file.proc.fs.m.device.RUnlock()

	file.proc.m_filp.Lock()
	defer file.proc.m_filp.Unlock()

	file.close()
	return nil
}
