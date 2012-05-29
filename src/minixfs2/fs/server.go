package fs

import (
	"encoding/binary"
	"log"
	"minixfs2/alloctbl"
	"minixfs2/bcache"
	. "minixfs2/common"
	"minixfs2/device"
	"minixfs2/inode"
)

type FileSystem struct {
	devices []BlockDevice // the devices attached to the file system
	devinfo []*DeviceInfo // alloc tables and device parameters

	bcache BlockCache // the block cache for all devices
	itable InodeTbl   // the shared inode table

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
func NewFileSystem(dev BlockDevice) (*FileSystem, *Process, error) {
	// Check to make sure we have a valid device
	devinfo, err := GetDeviceInfo(dev)
	if err != nil {
		return nil, nil, err
	}

	fs := new(FileSystem)

	fs.devices = make([]BlockDevice, NR_DEVICES)
	fs.devinfo = make([]*DeviceInfo, NR_DEVICES)

	fs.bcache = bcache.NewLRUCache(NR_DEVICES, NR_BUFS, NR_BUF_HASH)
	fs.itable = inode.NewCache(fs.bcache, NR_DEVICES, NR_INODES)

	devinfo.Devnum = ROOT_DEVICE
	devinfo.AllocTbl = alloctbl.NewAllocTbl(devinfo, fs.bcache, ROOT_DEVICE)

	fs.devices[ROOT_DEVICE] = dev
	fs.devinfo[ROOT_DEVICE] = devinfo

	fs.procs = make(map[int]*Process, NR_PROCS)

	fs.in = make(chan reqFS)
	fs.out = make(chan resFS)

	if err := fs.bcache.MountDevice(ROOT_DEVICE, dev, devinfo); err != nil {
		log.Printf("Could not mount root device: %s", err)
		return nil, nil, err
	}
	fs.itable.MountDevice(ROOT_DEVICE, devinfo)

	// Fetch the root inode
	rip, err := fs.itable.GetInode(ROOT_DEVICE, ROOT_INODE)
	if err != nil {
		log.Printf("Failed to fetch root inode: %s", err)
		return nil, nil, err
	}

	// Create the root process
	fs.procs[ROOT_PROCESS] = &Process{
		ROOT_PROCESS,
		022,
		rip,
		rip,
		make([]*filp, OPEN_MAX),
		fs,
	}

	// Initialite the pidcounter
	fs.pidcounter = ROOT_PROCESS + 1

	go fs.loop()

	return fs, fs.procs[ROOT_PROCESS], nil
}

func (fs *FileSystem) loop() {
	alive := true
	for alive {
		req := <-fs.in
		switch req := req.(type) {
		case req_FS_Mount:
			fs.do_mount(req.proc, req.dev, req.path)
		case req_FS_Unmount:
			fs.do_unmount(req.proc, req.path)
		case req_FS_Sync:
			// Code here
		case req_FS_Shutdown:
			succ := fs.do_shutdown()
			if succ {
				alive = false
			}
		case req_FS_Fork:
			fs.do_fork(req.proc)
		case req_FS_Exit:
			fs.do_exit(req.proc)
		case req_FS_OpenCreat:
			fs.do_open(req.proc, req.path, req.flags, req.mode)
		case req_FS_Close:
			fs.do_close(req.proc, req.fd)
		case req_FS_Stat:
			// Code here
		case req_FS_Chmod:
			// Code here
		case req_FS_Link:
			// Code here
		case req_FS_Unlink:
			fs.do_unlink(req.proc, req.path)
		case req_FS_Mkdir:
			// Code here
		case req_FS_Rmdir:
			// Code here
		case req_FS_Chdir:
			fs.do_chdir(req.proc, req.path)
		}
	}
}
