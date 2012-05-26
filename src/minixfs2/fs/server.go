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

	procs []*Process // the list of user processes

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

	fs.procs = make([]*Process, NR_PROCS)

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
		make([]*Filp, OPEN_MAX),
		fs,
	}

	go fs.loop()

	return fs, fs.procs[ROOT_PROCESS], nil
}

func (fs *FileSystem) loop() {
	alive := true
	for alive {
		req := <-fs.in
		switch req := req.(type) {
		case req_FS_Mount:
		if req.dev == nil {
				fs.out <- res_FS_Mount{EINVAL}
				continue
			}

			// scan bitmap block table to see if 'dev' is already mounted
			found := false
			freeIndex := -1
			for i := 0; i < NR_DEVICES; i++ {
				if fs.devices[i] == req.dev {
					found = true
				} else if fs.devices[i] == nil {
					freeIndex = i
				}
			}

			if found {
				fs.out <- res_FS_Mount{EBUSY} // already mounted
				continue
			}

			if freeIndex == -1 {
				fs.out <- res_FS_Mount{ENFILE} // no device slot available
				continue
			}

			// Invalidate the cache for this index to be sure
			fs.bcache.Invalidate(freeIndex)

			// Fill in the device info
			devinfo, err := GetDeviceInfo(req.dev)

			// If it a recognized Minix filesystem
			if err != nil {
				fs.out <- res_FS_Mount{err}
				continue
			}

			// Create a new allocation table for this device
			alloc := alloctbl.NewAllocTbl(devinfo, fs.bcache, freeIndex)

			// Update the device number/alloc table
			devinfo.Devnum = freeIndex
			devinfo.AllocTbl = alloc

			// Add the device to the block cache/inode table
			fs.bcache.MountDevice(freeIndex, req.dev, devinfo)
			fs.itable.MountDevice(freeIndex, devinfo)
			fs.devices[freeIndex] = req.dev
			fs.devinfo[freeIndex] = devinfo

			// Get the inode of the file to be mounted on
			rip, err := fs.eatPath(fs.procs[ROOT_PROCESS], req.path)

			if err != nil {
				// Perform lots of cleanup
				fs.devices[freeIndex] = nil
				fs.devinfo[freeIndex] = nil
				fs.bcache.UnmountDevice(freeIndex)
				fs.itable.UnmountDevice(freeIndex)
				fs.out <- res_FS_Mount{err}
				continue
			}

			var r error = nil

			// It may not be busy
			if rip.Count > 1 {
				r = EBUSY
			}

			// It may not be spacial
			bits := rip.Type()
			if bits == I_BLOCK_SPECIAL || bits == I_CHAR_SPECIAL {
				r = ENOTDIR
			}

			// Get the root inode of the mounted file system
			var root_ip *Inode
			if r == nil {
				root_ip, err = fs.itable.GetInode(freeIndex, ROOT_INODE)
				if err != nil {
					r = err
				}
			}

			if root_ip != nil && root_ip.Mode == 0 {
				r = EINVAL
			}

			// File types of 'rip' and 'root_ip' may not conflict
			if r == nil {
				if !rip.IsDirectory() && root_ip.IsDirectory() {
					r = EISDIR
				}
			}

			// If error, return the bitmap and both inodes; release the maps
			if r != nil {
				// TODO: Refactor this error handling code?
				// Perform lots of cleanup
				fs.devices[freeIndex] = nil
				fs.devinfo[freeIndex] = nil
				fs.bcache.UnmountDevice(freeIndex)
				fs.itable.UnmountDevice(freeIndex)
				fs.out <- res_FS_Mount{r}
				continue
			}

			// Nothing else can go wrong, so perform the mount
			minfo := &MountInfo{
				MountPoint:  rip,
				MountTarget: root_ip,
			}
			rip.Mounted = minfo     // so we can find the root inode during lookup
			root_ip.Mounted = minfo // so we can easily resolve from a mount target to the mount point
			fs.out <- res_FS_Mount{nil}
		case req_FS_Unmount:
			// The filesystem hierarchy cannot change during the processing of
			// this request. We're going to use a bit of a hack here,
			// returning the inode and then continuing to use it.
			rip, err := fs.eatPath(req.proc, req.path)
			if err != nil {
				fs.out <- res_FS_Unmount{err}
				continue
			}

			devIndex := rip.Devinfo.Devnum
			fs.itable.PutInode(rip)

			// See if the mounted device is busy. Only one inode using it should be
			// open, the root inode, and only once.

			if fs.itable.IsDeviceBusy(devIndex) {
				fs.out <- res_FS_Unmount{EBUSY} // can't unmount a busy file system
				continue
			}

			if rip.Mounted == nil {
				// This is not a mounted file system
				fs.out <- res_FS_Unmount{EINVAL}
				continue
			}

			minfo := rip.Mounted

			// Clear each inode of the mount info
			minfo.MountPoint.Mounted = nil
			minfo.MountTarget.Mounted = nil

			// Release each inode
			fs.itable.PutInode(minfo.MountPoint)
			fs.itable.PutInode(minfo.MountTarget)

			// Flush and invalidate the cache for the device
			fs.bcache.Flush(devIndex)
			fs.bcache.Invalidate(devIndex)

			// TODO: Shut down the bitmap process?
			fs.devices[devIndex] = nil
			fs.devinfo[devIndex] = nil
			fs.bcache.UnmountDevice(devIndex)
			fs.itable.UnmountDevice(devIndex)
			fs.out <- res_FS_Unmount{nil}
		case req_FS_Sync:
			// Code here
		case req_FS_Shutdown:
			fs.out <- res_FS_Shutdown{}
		case req_FS_Fork:
			// Code here
		case req_FS_Exit:
			fs.out <- res_FS_Exit{}
		case req_FS_Open:
			// Code here
		case req_FS_Creat:
			// Code here
		case req_FS_Close:
			// Code here
		case req_FS_Stat:
			// Code here
		case req_FS_Chmod:
			// Code here
		case req_FS_Link:
			// Code here
		case req_FS_Unlink:
			// Code here
		case req_FS_Mkdir:
			// Code here
		case req_FS_Rmdir:
			// Code here
		case req_FS_Chdir:
			// Code here
		}
	}
}
