package minixfs

import "encoding/binary"
import "log"
import "os"

// FileSystem encapsulates a minix file system. The interface provided by the
// exported methods is thread-safe by ensuring that only one file-system level
// system call may occur at a time.
type FileSystem struct {
	devs   []BlockDevice // the block devices that comprise the file system
	supers []*Superblock // the superblocks for the given devices

	// These two members are individually locked and protected, although the
	// icache can call into fs.get_block specifically.
	cache  BlockCache  // the block cache (shared across all devices)
	icache *InodeCache // the inode cache (shared across all devices)

	_filp  []*filp    // the filp table
	_procs []*Process // an array of processes that have been opened

	in  chan m_fs_req
	out chan m_fs_res
}

// Create a new FileSystem from a given file on the filesystem
func OpenFileSystemFile(filename string) (*FileSystem, os.Error) {
	dev, err := NewFileDevice(filename, binary.LittleEndian)

	if err != nil {
		return nil, err
	}

	return NewFileSystem(dev)
}

// Create a new FileSystem from a given file on the filesystem
func NewFileSystem(dev BlockDevice) (*FileSystem, os.Error) {
	var fs *FileSystem = new(FileSystem)

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
		make([]*File, OPEN_MAX)}

	fs.in = make(chan m_fs_req)
	fs.out = make(chan m_fs_res)

	go fs.loop()

	return fs, nil
}

func (fs *FileSystem) loop() {
	var in <-chan m_fs_req = fs.in
	var out chan<- m_fs_res = fs.out

	for req := range in {
		switch req := req.(type) {
		case m_fs_req_close:
			err := fs.close()
			out <- m_fs_res_err{err}
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

func (fs *FileSystem) Close() os.Error {
	fs.in <- m_fs_req_close{}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *FileSystem) Mount(dev BlockDevice, path string) os.Error {
	fs.in <- m_fs_req_mount{dev, path}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *FileSystem) Unmount(dev BlockDevice) os.Error {
	fs.in <- m_fs_req_unmount{dev}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *FileSystem) Spawn(pid int, umask uint16, rootpath string) (*Process, os.Error) {
	fs.in <- m_fs_req_spawn{pid, umask, rootpath}
	res := (<-fs.out).(m_fs_res_spawn)
	return res.proc, res.err
}

func (fs *FileSystem) Exit(proc *Process) {
	fs.in <- m_fs_req_exit{proc}
	<-fs.out
	return
}

func (fs *FileSystem) Open(proc *Process, path string, flags int, mode uint16) (*File, os.Error) {
	fs.in <- m_fs_req_open{proc, path, flags, mode}
	res := (<-fs.out).(m_fs_res_open)
	return res.file, res.err
}

func (fs *FileSystem) Unlink(proc *Process, path string) os.Error {
	fs.in <- m_fs_req_unlink{proc, path}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *FileSystem) Mkdir(proc *Process, path string, mode uint16) os.Error {
	fs.in <- m_fs_req_mkdir{proc, path, mode}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *FileSystem) Rmdir(proc *Process, path string) os.Error {
	fs.in <- m_fs_req_rmdir{proc, path}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}

func (fs *FileSystem) Chdir(proc *Process, path string) os.Error {
	fs.in <- m_fs_req_chdir{proc, path}
	res := (<-fs.out).(m_fs_res_err)
	return res.err
}
