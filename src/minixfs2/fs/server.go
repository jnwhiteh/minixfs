package fs

import (
	"minixfs2/alloctbl"
	. "minixfs2/common"
)

type FileSystem struct {
	devices []BlockDevice
	devinfo []*DeviceInfo
	bcache  BlockCache
	itable  InodeTbl
	procs []*Process

	in  chan reqFS
	out chan resFS
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
			rip.Mounted = root_ip // this inode has root_ip mounted on top of it
			fs.out <- res_FS_Mount{nil}
		case req_FS_Unmount:
			// Code here
		case req_FS_Sync:
			// Code here
		case req_FS_Shutdown:
			// Code here
		case req_FS_Fork:
			// Code here
		case req_FS_Exit:
			// Code here
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
		default:
			// This can be removed when you utilize 'req'
			_ = req
		}
	}
}
