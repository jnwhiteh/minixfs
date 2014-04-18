package fs

import (
	"encoding/binary"
	"log"
	"github.com/jnwhiteh/minixfs/alloctbl"
	"github.com/jnwhiteh/minixfs/bcache"
	"github.com/jnwhiteh/minixfs/common"
	"github.com/jnwhiteh/minixfs/device"
	"github.com/jnwhiteh/minixfs/inode"
)

type FileSystem struct {
	devices []common.BlockDevice // the devices attached to the file system
	devinfo []*common.DeviceInfo // alloc tables and device parameters

	bcache common.BlockCache // the block cache for all devices
	itable common.InodeTbl   // the shared inode table

	procs      map[int]*Process // the list of user processes
	pidcounter int              // the next available pid

	in  chan reqFS
	out chan resFS
}

// Create a new FileSystem from a given file on the filesystem
func OpenFileSystemFile(filename string) (*FileSystem, *Process, error) {
	dev, err := device.NewFileDevice(filename, binary.LittleEndian)

	if err != nil {
		return nil, nil, err
	}

	return NewFileSystem(dev)
}

// Create a new FileSystem from a given file on the filesystem
func NewFileSystem(dev common.BlockDevice) (*FileSystem, *Process, error) {
	// Check to make sure we have a valid device
	devinfo, err := common.GetDeviceInfo(dev)
	if err != nil {
		return nil, nil, err
	}

	fs := new(FileSystem)

	fs.devices = make([]common.BlockDevice, common.NR_DEVICES)
	fs.devinfo = make([]*common.DeviceInfo, common.NR_DEVICES)

	fs.bcache = bcache.NewLRUCache(common.NR_DEVICES, common.NR_BUFS, common.NR_BUF_HASH)
	fs.itable = inode.NewCache(fs.bcache, common.NR_DEVICES, common.NR_INODES)

	devinfo.Devnum = common.ROOT_DEVICE

	if err := fs.bcache.MountDevice(common.ROOT_DEVICE, dev, devinfo); err != nil {
		log.Printf("Could not mount root device: %s", err)
		return nil, nil, err
	}
	fs.itable.MountDevice(common.ROOT_DEVICE, devinfo)

	devinfo.AllocTbl = alloctbl.NewAllocTbl(devinfo, fs.bcache, common.ROOT_DEVICE)

	fs.devices[common.ROOT_DEVICE] = dev
	fs.devinfo[common.ROOT_DEVICE] = devinfo

	fs.procs = make(map[int]*Process, common.NR_PROCS)

	fs.in = make(chan reqFS)
	fs.out = make(chan resFS)

	// Fetch the root inode
	rip, err := fs.itable.GetInode(common.ROOT_DEVICE, common.ROOT_INODE)
	if err != nil {
		log.Printf("Failed to fetch root inode: %s", err)
		return nil, nil, err
	}

	// Create the root process
	fs.procs[common.ROOT_PROCESS] = &Process{
		common.ROOT_PROCESS,
		022,
		rip,
		rip,
		make([]*filp, common.OPEN_MAX),
		fs,
	}

	// Initialite the pidcounter
	fs.pidcounter = common.ROOT_PROCESS + 1

	go fs.loop()

	return fs, fs.procs[common.ROOT_PROCESS], nil
}

func (fs *FileSystem) loop() {
	alive := true
	for alive {
		req := <-fs.in
		switch req := req.(type) {
		case req_FS_Mount:
			err := fs.do_mount(req.proc, req.dev, req.path)
			fs.out <- res_FS_Mount{err}
		case req_FS_Unmount:
			err := fs.do_unmount(req.proc, req.path)
			fs.out <- res_FS_Unmount{err}
		case req_FS_Sync:
			// Code here
		case req_FS_Shutdown:
			err := fs.do_shutdown()
			if err == nil {
				alive = false
			}
			fs.out <- res_FS_Shutdown{err}
		case req_FS_Fork:
			proc, err := fs.do_fork(req.proc)
			fs.out <- res_FS_Fork{proc, err}
		case req_FS_Exit:
			fs.do_exit(req.proc)
			fs.out <- res_FS_Exit{}
		case req_FS_OpenCreat:
			fd, err := fs.do_open(req.proc, req.path, req.flags, req.mode)
			fs.out <- res_FS_OpenCreat{fd, err}
		case req_FS_Close:
			err := fs.do_close(req.proc, req.fd)
			fs.out <- res_FS_Close{err}
		case req_FS_Stat:
			// Code here
		case req_FS_Chmod:
			// Code here
		case req_FS_Link:
			err := fs.do_link(req.proc, req.oldpath, req.newpath)
			fs.out <- res_FS_Link{err}
		case req_FS_Unlink:
			err := fs.do_unlink(req.proc, req.path)
			fs.out <- res_FS_Unlink{err}
		case req_FS_Mkdir:
			err := fs.do_mkdir(req.proc, req.path, req.mode)
			fs.out <- res_FS_Mkdir{err}
		case req_FS_Rmdir:
			err := fs.do_rmdir(req.proc, req.path)
			fs.out <- res_FS_Rmdir{err}
		case req_FS_Chdir:
			err := fs.do_chdir(req.proc, req.path)
			fs.out <- res_FS_Chdir{err}
		}
	}
}
