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

	// These two members are individually locked and protected, although the
	// icache can call into fs.get_block specifically.
	cache  BlockCache  // the block cache (shared across all devices)
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
		make([]*File, OPEN_MAX),
		new(sync.RWMutex)}

	fs.m.device = new(sync.RWMutex)
	fs.m.procs = new(sync.RWMutex)
	fs.m.filp = new(sync.RWMutex)

	return fs, nil
}

// Close the filesystem
func (fs *FileSystem) Close() (err os.Error) {
	fs.m.device.Lock()
	defer fs.m.device.Unlock()

	devs := fs.devs
	supers := fs.supers

	// Unmount each non-root device
	for i := ROOT_DEVICE + 1; i < NR_SUPERS; i++ {
		if devs[i] != nil {
			fs.cache.Flush(i)
			WriteSuperblock(devs[i], supers[i]) // flush the superblock

			err = fs.do_unmount(devs[i])
			if err != nil {
				return err
			}
		}
	}

	// Unmount the root device
	if fs.icache.IsDeviceBusy(ROOT_DEVICE) {
		// Cannot unmount this device, so we need to fail
		return EBUSY
	} else {
		fs.cache.Flush(ROOT_DEVICE)
		WriteSuperblock(devs[ROOT_DEVICE], supers[ROOT_DEVICE])
		fs.devs[ROOT_DEVICE].Close()
	}

	return nil
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
func (fs *FileSystem) get_block(dev, bnum int, btype BlockType, only_search int) *CacheBlock {
	return fs.cache.GetBlock(dev, bnum, btype, only_search)
}

// The put_block method is a wrapper for fs.cache.PutBlock()
func (fs *FileSystem) put_block(bp *CacheBlock, btype BlockType) {
	fs.cache.PutBlock(bp, btype)
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
