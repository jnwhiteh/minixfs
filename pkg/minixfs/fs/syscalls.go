package fs

import (
	. "../../minixfs/common/_obj/minixfs/common"
	"../../minixfs/utils/_obj/minixfs/utils"
	"../bitmap/_obj/minixfs/bitmap"
	"log"
	"os"
	"sync"
)

// Shutdown the file system by umounting all of the mounted devices.
func (fs *FileSystem) Shutdown() (err os.Error) {
	// Acquire the device lock
	fs.m.device.Lock()
	defer fs.m.device.Unlock()

	devices := fs.devices

	// Unmount each non-root device
	for i := ROOT_DEVICE + 1; i < NR_DEVICES; i++ {
		if devices[i] != nil {
			fs.bcache.Flush(i)
			// TODO: Flush the superblock here?

			// Unmount the device
			err = fs.unmount(i) // this closes the device and processes
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
		if err := fs.unmount(ROOT_DEVICE); err != nil {
			log.Printf("Error unmounting root device: %s", err)
		}
		if err := fs.bcache.Close(); err != nil {
			log.Printf("Error closing block cache: %s", err)
		}
		if err := fs.icache.Close(); err != nil {
			log.Printf("Error closing inode cache: %s", err)
		}
	}

	return nil
}

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
	fs.mountinfo[freeIndex] = mountInfo{rip, root_ip}

	return nil
}

func (fs *FileSystem) Unmount(dev RandDevice) os.Error {
	fs.m.device.Lock()
	defer fs.m.device.Unlock()

	// Find the numeric index of the device
	devIndex := -1

	for i := 0; i < NR_DEVICES; i++ {
		if fs.devices[i] == dev {
			devIndex = i
			break
		}
	}

	if devIndex == -1 {
		return EINVAL
	}

	return fs.unmount(devIndex)
}

var ERR_PID_EXISTS = os.NewError("Process already exists")
var ERR_PATH_LOOKUP = os.NewError("Could not lookup path")

// Spawn a new process
func (fs *FileSystem) Spawn(pid int, umask uint16, rootpath string) (*Process, os.Error) {
	fs.m.device.RLock()
	defer fs.m.device.RUnlock()
	fs.m.proc.Lock()
	defer fs.m.proc.Unlock()

	if fs.procs[pid] != nil {
		return nil, ERR_PID_EXISTS
	}

	// Get an inode from a path
	rip, err := fs.eatPath(fs.procs[ROOT_PROCESS], rootpath)
	if err != nil {
		return nil, err
	}

	rinode := rip
	winode := rinode
	filp := make([]*Filp, OPEN_MAX)
	files := make([]*File, OPEN_MAX)
	umask = ^umask // convert it so its actually usable as a mask
	mutex := new(sync.Mutex)

	proc := &Process{pid, umask, rinode, winode, filp, files, mutex}
	fs.procs[pid] = proc
	return proc, nil
}

// Exit a spawned process.
func (fs *FileSystem) Exit(proc *Process) {
	// We'll be changing both the process itself and the process table, so
	// make sure they are properly acquired
	fs.m.device.RLock()
	defer fs.m.device.RUnlock()
	fs.m.proc.Lock()
	defer fs.m.proc.Unlock()
	proc.m.Lock()
	defer proc.m.Unlock()

	// For each file that is open, close it
	for i := 0; i < len(proc.files); i++ {
		if proc.files[i] != nil {
			file := proc.files[i]
			fs.close(proc, file)
		}
	}

	fs.procs[proc.pid] = nil

	if proc.workdir != proc.rootdir {
		fs.icache.PutInode(proc.workdir)
	}
	fs.icache.PutInode(proc.rootdir)
}

var mode_map = []uint16{R_BIT, W_BIT, R_BIT | W_BIT, 0}

func (fs *FileSystem) Open(proc *Process, path string, oflags int, omode uint16) (*File, os.Error) {
	// Remap the bottom two bits of oflags
	bits := mode_map[oflags&O_ACCMODE]

	var err os.Error = nil
	var rip *CacheInode = nil
	var exist bool = false

	// If O_CREATE is set, try to make the file
	if oflags&O_CREAT > 0 {
		// Create a new node by calling new_node()
		omode := I_REGULAR | (omode & ALL_MODES & proc.umask)
		rip, err = fs.newNode(proc, path, omode, NO_ZONE)
		if err == nil {
			exist = false
		} else if err != EEXIST {
			return nil, err
		} else {
			exist = (oflags&O_EXCL == 0)
		}
	} else {
		// scan path name
		rip, err = fs.eatPath(proc, path)
		if err != nil {
			return nil, err
		}
	}

	// Acquire the filp table and process mutexes
	fs.m.filp.Lock()
	defer fs.m.filp.Unlock()
	proc.m.Lock()
	defer proc.m.Unlock()

	// Find an available file descriptor/filp entry
	fd := -1
	filpidx := -1

	for i := 0; i < OPEN_MAX; i++ {
		if proc.filp[i] == nil {
			fd = i
			break
		}
	}

	if fd < 0 {
		return nil, EMFILE
	}

	for i := 0; i < NR_FILPS; i++ {
		if fs.filps[i] == nil {
			filpidx = i
			break
		}
	}

	if filpidx < 0 {
		return nil, ENFILE
	}

	var filp *Filp

	err = nil
	if exist {
		// TODO: Check permissions
		switch rip.GetType() {
		case I_REGULAR:
			if oflags&O_TRUNC > 0 {
				utils.Truncate(rip, rip.Bitmap, fs.bcache)
				fs.wipeInode(rip)
				// Send the inode from the inode cache to the block cache, so
				// it gets written on the next cache flush
				fs.icache.FlushInode(rip)
			}
		case I_DIRECTORY:
			// Directories may be read, but not written
			if bits&W_BIT > 0 {
				err = EISDIR
			}
		default:
			// TODO: Add other cases
			panic("NYI: Process.Open with non regular/directory")
		}
	}

	if err != nil {
		// Something went wrong
		return nil, err
	} else {
		// Allocate a proper filp entry and update fs/filp tables
		filp = &Filp{bits, oflags, rip, 1, 0}
		proc.filp[fd] = filp
		fs.filps[filpidx] = filp
	}

	file := &File{filp, fd}
	proc.files[fd] = file
	return file, nil
}
