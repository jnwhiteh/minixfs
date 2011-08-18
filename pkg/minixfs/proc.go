package minixfs

import (
	"math"
	"os"
)

// Skeleton implementation of system calls required for tests in 'fs_test.go'
type Process struct {
	fs      *FileSystem // the file system on which this process resides
	pid     int         // numeric id of the process
	umask   uint16      // file creation mask
	rootdir *Inode      // root directory of the process
	workdir *Inode      // working directory of the process
	_filp   []*filp     // the list of file descriptors
	_files  []*File     // the list of open files
}

func (proc *Process) Exit() {
	fs := proc.fs

	// For each file that is open, close it
	for i := 0; i < len(proc._files); i++ {
		if proc._files[i] != nil {
			file := proc._files[i]
			file.close()
		}
	}

	fs._procs[proc.pid] = nil

	if proc.workdir != proc.rootdir {
		fs.put_inode(proc.workdir)
	}
	fs.put_inode(proc.rootdir)
}

var mode_map = []uint16{R_BIT, W_BIT, R_BIT | W_BIT, 0}

// NewProcess acquires the 'fs.filp' lock in write mode.
func (proc *Process) Open(path string, oflags int, omode uint16) (*File, os.Error) {
	// Remap the bottom two bits of oflags
	bits := mode_map[oflags&O_ACCMODE]

	var err os.Error = nil
	var rip *Inode = nil
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
		proc._filp[fd] = nil
		proc.fs._filp[filpidx] = nil

		return nil, err
	} else {
		// Allocate a proper filp entry and update fs/filp tables
		filp = NewFilp(bits, oflags, rip, 1, 0)
		proc._filp[fd] = filp
		proc.fs._filp[filpidx] = filp
	}

	file := &File{filp, proc, fd}
	proc._files[fd] = file
	return file, nil
}

func (proc *Process) Unlink(path string) os.Error {
	fs := proc.fs
	// Call a helper function to do most of the dirty work for us
	rldirp, rip, rest, err := fs.unlink(proc, path)
	if err != nil || rldirp == nil || rip == nil {
		return err
	}

	// Now test if the call is allowed (altered from Minix)
	if rip.inum == ROOT_INODE {
		err = EBUSY
	}
	if err == nil {
		err = fs.unlink_file(rldirp, rip, rest)
	}

	// If unlink was possible, it has been done, otherwise it has not
	fs.put_inode(rip)
	fs.put_inode(rldirp)
	return err
}

// Perform the mkdir(name, mode) system call
func (proc *Process) Mkdir(path string, mode uint16) os.Error {
	fs := proc.fs
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

	var rip *Inode

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

func (proc *Process) Rmdir(path string) os.Error {
	fs := proc.fs
	// Call a helper function to do most of the dirty work for us
	rldirp, rip, rest, err := fs.unlink(proc, path)
	if err != nil || rldirp == nil || rip == nil {
		return err
	}

	err = fs.remove_dir(proc, rldirp, rip, rest) // perform the rmdir

	// If unlink was possible, it has been done, otherwise it has not
	fs.put_inode(rip)
	fs.put_inode(rldirp)
	return err
}

func (proc *Process) Chdir(path string) os.Error {
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
