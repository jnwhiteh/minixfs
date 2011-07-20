package minixfs

import (
	"os"
)

// Mount the filesystem on 'dev' at 'path' in the root filesystem
func (fs *FileSystem) do_mount(dev BlockDevice, path string) os.Error {
	// argument check
	if dev == nil {
		return EINVAL
	}

	// scan super block table to see if 'dev' is already mounted
	found := false
	freeIndex := -1
	for i := 0; i < NR_SUPERS; i++ {
		if fs.devs[i] == dev {
			found = true
		} else if fs.devs[i] == nil {
			freeIndex = i
		}
	}

	if found {
		return EBUSY // already mounted
	}

	if freeIndex == -1 {
		return ENFILE // no super block available
	}

	fs.do_sync()
	fs.cache.Invalidate(freeIndex)

	// Fill in the super block
	sp, err := ReadSuperblock(dev)

	// If it a recognized Minix filesystem
	if err != nil {
		dev.Close()
		return err
	}

	// Add the super/dev to the the filesystem (will need to be cleared if
	// there is a problem)
	fs.devs[freeIndex] = dev
	fs.supers[freeIndex] = sp
	fs.cache.MountDevice(freeIndex, dev, sp)
	fs.icache.MountDevice(freeIndex, dev, sp)

	// Get the inode of the file to be mounted on
	fs.m.procs.RLock()
	rip, err := fs.eat_path(fs._procs[ROOT_PROCESS], path)
	fs.m.procs.RUnlock()

	if err != nil {
		fs.devs[freeIndex] = nil
		fs.supers[freeIndex] = nil
		return err
	}

	var r os.Error = nil

	// It may not be busy
	if rip.Count() > 1 {
		r = EBUSY
	}

	// It may not be spacial
	bits := rip.GetType()
	if bits == I_BLOCK_SPECIAL || bits == I_CHAR_SPECIAL {
		r = ENOTDIR
	}

	// Get the root inode of the mounted file system
	var root_ip *Inode
	if r == nil {
		root_ip, err = fs.get_inode(freeIndex, ROOT_INODE)
		if err != nil {
			r = err
		}
	}

	if root_ip != nil && root_ip.Mode() == 0 {
		r = EINVAL
	}

	// File types of 'rip' and 'root_ip' may not conflict
	if r == nil {
		mdir := rip.GetType() == I_DIRECTORY
		rdir := root_ip.GetType() == I_DIRECTORY
		if !mdir && rdir {
			r = EISDIR
		}
	}

	// If error, return the super block and both inodes; release the maps
	if r != nil {
		fs.put_inode(rip)
		fs.put_inode(root_ip)
		fs.do_sync()
		fs.cache.Invalidate(freeIndex)
		fs.devs[freeIndex] = nil
		fs.supers[freeIndex] = nil
		fs.cache.UnmountDevice(freeIndex)
		fs.icache.UnmountDevice(freeIndex)
		dev.Close()
		return r
	}

	// Nothing else can go wrong, so perform the mount
	rip.SetMount(true)
	sp.imount = rip
	sp.isup = root_ip
	return nil
}

// Unmount a given block device
func (fs *FileSystem) do_unmount(dev BlockDevice) os.Error {
	// Determine the numeric index of this device
	devnum := -1
	for i := 0; i < NR_SUPERS; i++ {
		if fs.devs[i] == dev {
			devnum = i
			break
		}
	}

	// See if the mounted device is busy. Only 1 inode using it should be open
	// -- the root inode -- and that inode only 1 time.
	if fs.icache.IsDeviceBusy(devnum) {
		return EBUSY // can't unmount a busy file system
	}

	// Find the super block
	sp := fs.supers[devnum]

	// Sync the disk and invalidate the cache
	fs.do_sync()
	fs.cache.Flush(devnum)
	fs.cache.Invalidate(devnum)
	if sp == nil {
		return EINVAL
	}

	// Close the device the file system lives on
	fs.devs[devnum].Close()

	// Finish off the unmount
	sp.imount.SetMount(false) // inode returns to normal
	fs.put_inode(sp.imount)   // release the inode mounted on
	fs.put_inode(sp.isup)     // release the root inode of the mounted fs
	sp.imount = nil

	fs.devs[devnum] = nil
	fs.supers[devnum] = nil
	return nil
}
