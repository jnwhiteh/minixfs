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

	fs.icache = NewInodeCache(fs, NR_INODES)

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

func (fs *fileSystem) _AllocZone(dev int, zstart int) (int, os.Error) {
	fs.in <- m_fs_req_alloc_zone{dev, zstart}
	res := (<-fs.out).(m_fs_res_alloc_zone)
	return res.zone, res.err
}

func (fs *fileSystem) _FreeZone(dev int, zone int) {
	fs.in <- m_fs_req_free_zone{dev, zone}
	<-fs.out
	return
}
