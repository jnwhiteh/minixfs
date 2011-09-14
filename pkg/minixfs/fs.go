package minixfs

import "encoding/binary"
import "log"
import "os"

type FileSystem interface {
	Shutdown() os.Error
	Mount(dev BlockDevice, path string) os.Error
	Unmount(dev BlockDevice) os.Error
	Spawn(pid int, umask uint16, rootpath string) (*Process, os.Error)
	Exit(proc *Process)
	Open(proc *Process, path string, flags int, mode uint16) (*File, os.Error)
	Close(proc *Process, file *File) os.Error
	Unlink(proc *Process, path string) os.Error
	Mkdir(proc *Process, path string, mode uint16) os.Error
	Rmdir(proc *Process, path string) os.Error
	Chdir(proc *Process, path string) os.Error

	Seek(proc *Process, file *File, pos, whence int) (int, os.Error)
	Read(proc *Process, file *File, b []byte) (int, os.Error)
	Write(proc *Process, file *File, b []byte) (int, os.Error)
}

// FileSystem encapsulates a minix file system. The interface provided by the
// exported methods is thread-safe by ensuring that only one file-system level
// system call may occur at a time.
type fileSystem struct {
	devs   []BlockDevice // the block devices that comprise the file system
	supers []*Superblock // the superblocks for the given devices

	// These two members are individually locked and protected, although the
	// icache can call into fs.get_block specifically.
	cache  BlockCache  // the block cache (shared across all devices)
	icache *InodeCache // the inode cache (shared across all devices)

	filp    []*filp    // the filp table
	procs   []*Process // an array of processes that have been spawned
	finodes map[*Inode]*Finode

	in  chan m_fs_req
	out chan m_fs_res
}

// Create a new FileSystem from a given file on the filesystem
func OpenFileSystemFile(filename string) (*fileSystem, os.Error) {
	dev, err := NewFileDevice(filename, binary.LittleEndian)

	if err != nil {
		return nil, err
	}

	return NewFileSystem(dev)
}

// Create a new FileSystem from a given file on the filesystem
func NewFileSystem(dev BlockDevice) (*fileSystem, os.Error) {
	var fs *fileSystem = new(fileSystem)

	fs.cache = NewLRUCache()

	super, err := ReadSuperblock(dev)
	if err != nil {
		return nil, err
	}
	super.devno = ROOT_DEVICE
	super.cache = fs.cache

	fs.devs = make([]BlockDevice, NR_SUPERS)
	fs.supers = make([]*Superblock, NR_SUPERS)

	fs.icache = NewInodeCache(fs.cache, NR_INODES)

	fs.filp = make([]*filp, NR_FILPS)
	fs.procs = make([]*Process, NR_PROCS)

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

	fs.procs[ROOT_PROCESS] = &Process{fs, 0, 022, rip, rip,
		make([]*filp, OPEN_MAX),
		make([]*File, OPEN_MAX),
	}

	// TODO: Limit this?
	fs.finodes = make(map[*Inode]*Finode, OPEN_MAX)

	fs.in = make(chan m_fs_req)
	fs.out = make(chan m_fs_res)

	go fs.loop()

	return fs, nil
}

func (fs *fileSystem) loop() {
	var in <-chan m_fs_req = fs.in
	var out chan<- m_fs_res = fs.out

	for req := range in {
		switch req := req.(type) {
		case m_fs_req_shutdown:
			err := fs.shutdown()
			out <- m_fs_res_err{err}
			if err == nil {
				close(fs.in)
				close(fs.out)
			}
		case m_fs_req_mount:
			err := fs.mount(req.dev, req.path)
			out <- m_fs_res_err{err}
		case m_fs_req_unmount:
			err := fs.unmount(req.dev)
			out <- m_fs_res_err{err}
		case m_fs_req_spawn:
			proc, err := fs.spawn(req.pid, req.umask, req.rootpath)
			out <- m_fs_res_spawn{proc, err}
		case m_fs_req_exit:
			fs.exit(req.proc)
			out <- m_fs_res_empty{}
		case m_fs_req_open:
			file, err := fs.open(req.proc, req.path, req.flags, req.mode)
			out <- m_fs_res_open{file, err}
		case m_fs_req_close:
			err := fs.close(req.proc, req.file)
			out <- m_fs_res_err{err}
		case m_fs_req_unlink:
			err := fs.unlink(req.proc, req.path)
			out <- m_fs_res_err{err}
		case m_fs_req_mkdir:
			err := fs.mkdir(req.proc, req.path, req.mode)
			out <- m_fs_res_err{err}
		case m_fs_req_rmdir:
			err := fs.rmdir(req.proc, req.path)
			out <- m_fs_res_err{err}
		case m_fs_req_chdir:
			err := fs.chdir(req.proc, req.path)
			out <- m_fs_res_err{err}
		}
	}
}

func (fs *fileSystem) Shutdown() os.Error {
	fs.in <- m_fs_req_shutdown{}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *fileSystem) Mount(dev BlockDevice, path string) os.Error {
	fs.in <- m_fs_req_mount{dev, path}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *fileSystem) Unmount(dev BlockDevice) os.Error {
	fs.in <- m_fs_req_unmount{dev}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *fileSystem) Spawn(pid int, umask uint16, rootpath string) (*Process, os.Error) {
	fs.in <- m_fs_req_spawn{pid, umask, rootpath}
	res := (<-fs.out).(m_fs_res_spawn)
	return res.proc, res.err
}

func (fs *fileSystem) Exit(proc *Process) {
	fs.in <- m_fs_req_exit{proc}
	<-fs.out
	return
}

func (fs *fileSystem) Open(proc *Process, path string, flags int, mode uint16) (*File, os.Error) {
	fs.in <- m_fs_req_open{proc, path, flags, mode}
	res := (<-fs.out).(m_fs_res_open)
	return res.file, res.err
}

func (fs *fileSystem) Close(proc *Process, file *File) os.Error {
	fs.in <- m_fs_req_close{proc, file}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *fileSystem) Unlink(proc *Process, path string) os.Error {
	fs.in <- m_fs_req_unlink{proc, path}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *fileSystem) Mkdir(proc *Process, path string, mode uint16) os.Error {
	fs.in <- m_fs_req_mkdir{proc, path, mode}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *fileSystem) Rmdir(proc *Process, path string) os.Error {
	fs.in <- m_fs_req_rmdir{proc, path}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *fileSystem) Chdir(proc *Process, path string) os.Error {
	fs.in <- m_fs_req_chdir{proc, path}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

// Seek sets the position for the next read or write to pos, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end of the
// file. It returns the new offset and an Error, if any.
//
// TODO: Implement end of file seek and error checking
func (fs *fileSystem) Seek(proc *Process, file *File, pos int, whence int) (int, os.Error) {
	if file.fd == NO_FILE {
		return 0, EBADF
	}

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
func (fs *fileSystem) Read(proc *Process, file *File, b []byte) (int, os.Error) {
	if file.fd == NO_FILE {
		return 0, EBADF
	}

	n, err := file.finode.Read(b, file.Pos())
	file.SetPosDelta(n)

	return n, err
}

// Write a slice of bytes to the file at the current position. Returns the
// number of bytes actually written and an error (if any).
func (fs *fileSystem) Write(proc *Process, file *File, data []byte) (n int, err os.Error) {
	if file.fd == NO_FILE {
		return 0, EBADF
	}

	pos := file.Pos()
	// Check for O_APPEND flag
	if file.flags&O_APPEND > 0 {
		fsize := int(file.inode.Size())
		pos = fsize
	}

	n, err = file.finode.Write(data, pos)
	file.SetPos(pos + n)

	return n, err
}
