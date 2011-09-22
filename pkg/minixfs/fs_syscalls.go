package minixfs

import (
	"math"
	"os"
	"sync"
)

// Close the filesystem
func (fs *fileSystem) shutdown() (err os.Error) {
	devs := fs.devs
	supers := fs.supers

	// Unmount each non-root device
	for i := ROOT_DEVICE + 1; i < NR_SUPERS; i++ {
		if devs[i] != nil {
			dev := devs[i]

			fs.cache.Flush(i)
			WriteSuperblock(dev, supers[i]) // flush the superblock

			err = fs.do_unmount(dev) // this closes the device
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
		// TODO: Report errors?
		fs.devs[ROOT_DEVICE].Close()
		fs.supers[ROOT_DEVICE].Shutdown()
		fs.cache.Close()
	}

	return nil
}

// Mount the filesystem on 'dev' at 'path' in the root filesystem
func (fs *fileSystem) mount(dev RandDevice, path string) os.Error {
	return fs.do_mount(dev, path)
}

// Unmount a file system by device
func (fs *fileSystem) unmount(dev RandDevice) os.Error {
	return fs.do_unmount(dev)
}

var ERR_PID_EXISTS = os.NewError("Process already exists")
var ERR_PATH_LOOKUP = os.NewError("Could not lookup path")

func (fs *fileSystem) spawn(pid int, umask uint16, rootpath string) (*Process, os.Error) {
	if fs.procs[pid] != nil {
		return nil, ERR_PID_EXISTS
	}

	// Get an inode from a path
	rip, err := fs.eat_path(fs.procs[ROOT_PROCESS], rootpath)
	if err != nil {
		return nil, err
	}

	rinode := rip
	winode := rinode
	filp := make([]*filp, OPEN_MAX)
	files := make([]*File, OPEN_MAX)
	umask = ^umask // convert it so its actually usable as a mask

	proc := &Process{fs, pid, umask, rinode, winode, filp, files}
	fs.procs[pid] = proc
	return proc, nil
}

func (fs *fileSystem) exit(proc *Process) {
	// For each file that is open, close it
	for i := 0; i < len(proc._files); i++ {
		if proc._files[i] != nil {
			file := proc._files[i]
			fs.close(proc, file)
		}
	}

	fs.procs[proc.pid] = nil

	if proc.workdir != proc.rootdir {
		fs.put_inode(proc.workdir)
	}
	fs.put_inode(proc.rootdir)
}

var mode_map = []uint16{R_BIT, W_BIT, R_BIT | W_BIT, 0}

func (fs *fileSystem) open(proc *Process, path string, oflags int, omode uint16) (*File, os.Error) {
	// Remap the bottom two bits of oflags
	bits := mode_map[oflags&O_ACCMODE]

	var err os.Error = nil
	var rip *CacheInode = nil
	var exist bool = false

	// If O_CREATE is set, try to make the file
	if oflags&O_CREAT > 0 {
		// Create a new node by calling new_node()
		omode := I_REGULAR | (omode & ALL_MODES & proc.umask)
		rip, err = proc.fs.new_node(proc, path, omode, NO_ZONE)
		if err == nil {
			exist = false
		} else if err != EEXIST {
			return nil, err
		} else {
			exist = (oflags&O_EXCL == 0)
		}
	} else {
		// scan path name
		rip, err = proc.fs.eat_path(proc, path)
		if err != nil {
			return nil, err
		}
	}

	// Allocate a file descriptor and filp slot. This function will put a
	// static 'inUse' filp entry into both the fs/proc tables to prevent
	// re-allocation of the slot returned. As a result, if the open/creat
	// fails, this allocation needs to be reversed.
	fd, filpidx, err := proc.fs.reserve_fd(proc, 0, bits)
	var filp *filp

	err = nil
	if exist {
		// TODO: Check permissions
		switch rip.GetType() {
		case I_REGULAR:
			if oflags&O_TRUNC > 0 {
				proc.fs.truncate(rip)
				proc.fs.wipe_inode(rip)
				// Send the inode from the inode cache to the block cache, so
				// it gets written on the next cache flush
				proc.fs.icache.WriteInode(rip)
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
		// Something went wrong, release the filp reservation
		proc.filp[fd] = nil
		proc.fs.filp[filpidx] = nil

		return nil, err
	} else {
		// Allocate a proper filp entry and update fs/filp tables
		filp = NewFilp(bits, oflags, rip, 1, 0)
		proc.filp[fd] = filp
		proc.fs.filp[filpidx] = filp
	}

	// Check to see if we need to spawn a finode process
	finode, ok := fs.finodes[rip]
	if !ok {
		super := fs.supers[rip.dev]
		scale := super.Log_zone_size
		bsize := int(super.Block_size)
		maxsize := int(super.Max_size)

		finode = &Finode{rip, scale, bsize, maxsize, fs.cache, 0,
			make(chan m_finode_req),
			make(chan m_finode_res),
			new(sync.WaitGroup),
			nil,
		}
		go finode.loop()
		fs.finodes[rip] = finode, true
	}
	finode.count++

	file := &File{filp, proc, fd, finode}
	proc._files[fd] = file
	return file, nil
}

func (fs *fileSystem) close(proc *Process, file *File) os.Error {
	if file.fd == NO_FILE {
		return EBADF
	}

	// call the internal close function
	file.close()

	rip := file.inode
	finode, ok := fs.finodes[rip]
	if !ok {
		return EBADF
	}
	finode.count--
	if finode.count == 0 {
		finode.Close()
		fs.finodes[rip] = nil, false
	}

	return nil
}

func (fs *fileSystem) unlink(proc *Process, path string) os.Error {
	// Call a helper function to do most of the dirty work for us
	rldirp, rip, rest, err := fs.prep_unlink(proc, path)
	if err != nil || rldirp == nil || rip == nil {
		return err
	}

	//
	err = fs.unlink_file(rldirp, rip, rest)

	// If unlink was possible, it has been done, otherwise it has not
	fs.put_inode(rip)
	fs.put_inode(rldirp)
	return err
}

func (fs *fileSystem) mkdir(proc *Process, path string, mode uint16) os.Error {
	var dot, dotdot int
	var err_code os.Error

	// Check to see if it is possible to make another link in the parent
	// directory.
	ldirp, rest, err := fs.last_dir(proc, path) // pointer to new dir's parent
	if ldirp == nil {
		return err
	}
	if ldirp.Nlinks() >= math.MaxUint16 {
		fs.put_inode(ldirp)
		return EMLINK
	}

	var rip *CacheInode

	// Next make the inode. If that fails, return error code
	bits := I_DIRECTORY | (mode & RWX_MODES & proc.umask)
	rip, err_code = fs.new_node(proc, path, bits, 0)
	if rip == nil || err == EEXIST {
		fs.put_inode(rip)   // can't make dir: it already exists
		fs.put_inode(ldirp) // return parent too
		return err_code
	}

	// Get the inode numbers for . and .. to enter into the directory
	dotdot = int(ldirp.inum) // parent's inode number
	dot = int(rip.inum)      // inode number of the new dir itself

	// Now make dir entries for . and .. unless the disk is completely full.
	// Use dot1 and dot2 so the mode of the directory isn't important.
	rip.SetMode(bits)                                // set mode
	err1 := fs.search_dir(rip, ".", &dot, ENTER)     // enter . in the new dir
	err2 := fs.search_dir(rip, "..", &dotdot, ENTER) // enter .. in the new dir

	// If both . and .. were successfully entered, increment the link counts
	if err1 == nil && err2 == nil {
		// Normal case. it was possible to enter . and .. in the new dir
		rip.IncNlinks()      // this accounts for .
		ldirp.IncNlinks()    // this accounts for ..
		ldirp.SetDirty(true) // mark parent's inode as dirty
	} else {
		// It was not possible to enter . and .. or probably the disk was full
		nilinode := 0
		fs.search_dir(ldirp, rest, &nilinode, DELETE) // remove the new directory
		rip.DecNlinks()                               // undo the increment done in new_node
	}

	rip.SetDirty(true) // either way, Nlinks has changed

	fs.put_inode(ldirp)
	fs.put_inode(rip)
	return err_code
}

func (fs *fileSystem) rmdir(proc *Process, path string) os.Error {
	// Call a helper function to do most of the dirty work for us
	rldirp, rip, rest, err := fs.prep_unlink(proc, path)
	if err != nil || rldirp == nil || rip == nil {
		return err
	}

	err = fs.remove_dir(proc, rldirp, rip, rest) // perform the rmdir

	// If unlink was possible, it has been done, otherwise it has not
	fs.put_inode(rip)
	fs.put_inode(rldirp)
	return err
}

func (fs *fileSystem) chdir(proc *Process, path string) os.Error {
	rip, err := proc.fs.eat_path(proc, path)
	if rip == nil || err != nil {
		return err
	}

	var r os.Error

	if rip.GetType() != I_DIRECTORY {
		r = ENOTDIR
	}
	// TODO: Check permissions

	// If error then return inode
	if r != nil {
		proc.fs.put_inode(rip)
		return r
	}

	// Everything is OK. Make the change.
	proc.fs.put_inode(proc.workdir)
	proc.workdir = rip
	return nil
}
