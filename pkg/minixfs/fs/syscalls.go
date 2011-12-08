package fs

import (
	. "../../minixfs/common/_obj/minixfs/common"
	"../bitmap/_obj/minixfs/bitmap"
	"os"
)

// Mount the filesystem on 'dev' at 'path' in the root filesystem
func (fs *FileSystem) Mount(dev RandDevice, path string) os.Error {
	// argument check
	if dev == nil {
		return EINVAL
	}

	// Acquire and defer release of the device lock
	fs.m.device.Lock()
	defer fs.m.device.Unlock()

	// scan bitmap block table to see if 'dev' is already mounted
	found := false
	freeIndex := -1
	for i := 0; i < NR_DEVICES; i++ {
		if fs.devices[i] == dev {
			found = true
		} else if fs.devices[i] == nil {
			freeIndex = i
		}
	}

	if found {
		return EBUSY // already mounted
	}

	if freeIndex == -1 {
		return ENFILE // no device slot available
	}

	// Invalidate the cache, just to be sure
	fs.bcache.Invalidate(freeIndex)

	// Fill in the device info
	devinfo, err := GetDeviceInfo(dev)

	// If it a recognized Minix filesystem
	if err != nil {
		// Shut down device/bitmap
		dev.Close()
		return err
	}

	// Create a bitmap to handle allocation
	bmap := bitmap.NewBitmap(devinfo, fs.bcache, freeIndex)

	// Add the device/bitmap to the the filesystem (will need to be cleared if
	// there is a problem)
	fs.devices[freeIndex] = dev
	fs.bitmaps[freeIndex] = bmap
	fs.bcache.MountDevice(freeIndex, dev, devinfo)
	fs.icache.MountDevice(freeIndex, bmap, devinfo)

	// Get the inode of the file to be mounted on
	rip, err := fs.eatPath(fs.procs[ROOT_PROCESS], path)

	if err != nil {
		fs.devices[freeIndex] = nil
		fs.bitmaps[freeIndex] = nil
		// Shut down device/bitmap
		bmap.Close()
		dev.Close()
		return err
	}

	var r os.Error = nil

	// It may not be busy
	if rip.Count > 1 {
		r = EBUSY
	}

	// It may not be spacial
	bits := rip.Inode.Mode & I_TYPE
	if bits == I_BLOCK_SPECIAL || bits == I_CHAR_SPECIAL {
		r = ENOTDIR
	}

	// Get the root inode of the mounted file system
	var root_ip *CacheInode
	if r == nil {
		root_ip, err = fs.icache.GetInode(freeIndex, ROOT_INODE)
		if err != nil {
			r = err
		}
	}

	if root_ip != nil && root_ip.Inode.Mode == 0 {
		r = EINVAL
	}

	// File types of 'rip' and 'root_ip' may not conflict
	if r == nil {
		mdir := rip.IsDirectory()
		rdir := root_ip.IsDirectory()
		if !mdir && rdir {
			r = EISDIR
		}
	}

	// If error, return the bitmap and both inodes; release the maps
	if r != nil {
		fs.icache.PutInode(rip)
		fs.icache.PutInode(root_ip)
		fs.bcache.Invalidate(freeIndex)
		fs.devices[freeIndex] = nil
		fs.bitmaps[freeIndex] = nil
		fs.bcache.UnmountDevice(freeIndex)
		// TODO: Should there be a way to unmount from icache?
		//fs.icache.MountDevice(freeIndex, nil, nil)

		// Shut down device/bitmap
		dev.Close()
		bmap.Close()
		return r
	}

	// Nothing else can go wrong, so perform the mount
	rip.Mount = true

	// TODO: This is NYI
	/*
		sup.
			sp.imount = rip
			sp.isup = root_ip
			sp.devno = freeIndex
			sp.cache = fs.cache
	*/

	return nil
}
