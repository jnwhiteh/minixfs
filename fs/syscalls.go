package fs

import (
	"fmt"
	"log"
	"math"
	"github.com/jnwhiteh/minixfs/alloctbl"
	"github.com/jnwhiteh/minixfs/common"
	"github.com/jnwhiteh/minixfs/file"
	"sync"
)

func (fs *FileSystem) do_mount(proc *Process, dev common.BlockDevice, path string) error {
	if dev == nil {
		return common.EINVAL
	}

	// scan bitmap block table to see if 'dev' is already mounted
	found := false
	freeIndex := -1
	for i := 0; i < common.NR_DEVICES; i++ {
		if fs.devices[i] == dev {
			found = true
		} else if fs.devices[i] == nil {
			freeIndex = i
		}
	}

	if found {
		return common.EBUSY // already mounted
	}

	if freeIndex == -1 {
		return common.ENFILE // no device slot available
	}

	// Invalidate the cache for this index to be sure
	fs.bcache.Invalidate(freeIndex)

	// Fill in the device info
	devinfo, err :=common.GetDeviceInfo(dev)

	// If it a recognized Minix filesystem
	if err != nil {
		return err
	}

	// Create a new allocation table for this device
	alloc := alloctbl.NewAllocTbl(devinfo, fs.bcache, freeIndex)

	// Update the device number/alloc table
	devinfo.Devnum = freeIndex
	devinfo.AllocTbl = alloc

	// Add the device to the block cache/inode table
	fs.bcache.MountDevice(freeIndex, dev, devinfo)
	fs.itable.MountDevice(freeIndex, devinfo)
	fs.devices[freeIndex] = dev
	fs.devinfo[freeIndex] = devinfo

	// Get the inode of the file to be mounted on
	rip, err := fs.eatPath(fs.procs[common.ROOT_PROCESS], path)

	if err != nil {
		// Perform lots of cleanup
		fs.devices[freeIndex] = nil
		fs.devinfo[freeIndex] = nil
		fs.bcache.UnmountDevice(freeIndex)
		fs.itable.UnmountDevice(freeIndex)
		return err
	}

	var r error = nil

	// It may not be busy
	if rip.Count > 1 {
		r = common.EBUSY
	}

	// It may not be spacial
	bits := rip.Type()
	if bits == common.I_BLOCK_SPECIAL || bits == common.I_CHAR_SPECIAL {
		r = common.ENOTDIR
	}

	// Get the root inode of the mounted file system
	var root_ip *common.Inode
	if r == nil {
		root_ip, err = fs.itable.GetInode(freeIndex, common.ROOT_INODE)
		if err != nil {
			r = err
		}
	}

	if root_ip != nil && root_ip.Mode == 0 {
		r = common.EINVAL
	}

	// File types of 'rip' and 'root_ip' may not conflict
	if r == nil {
		if !rip.IsDirectory() && root_ip.IsDirectory() {
			r = common.EISDIR
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
		return r
	}

	// Nothing else can go wrong, so perform the mount
	minfo := &common.MountInfo{
		MountPoint:  rip,
		MountTarget: root_ip,
	}
	rip.Mounted = minfo     // so we can find the root inode during lookup
	root_ip.Mounted = minfo // so we can easily resolve from a mount target to the mount point

	// Store the mountinfo in the device info table for easy mapping
	devinfo.MountInfo = minfo
	return nil
}

func (fs *FileSystem) do_unmount(proc *Process, path string) error {
	// The filesystem hierarchy cannot change during the processing of
	// this request. We're going to use a bit of a hack here,
	// returning the inode and then continuing to use it.
	rip, err := fs.eatPath(proc, path)
	if err != nil {
		return err
	}

	devIndex := rip.Devinfo.Devnum
	fs.itable.PutInode(rip)

	// See if the mounted device is busy. Only one inode using it should be
	// open, the root inode, and only once.

	if fs.itable.IsDeviceBusy(devIndex) {
		return common.EBUSY // can't unmount a busy file system
	}

	if rip.Mounted == nil {
		return common.EINVAL // not a mounted file system
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

	// Shut down the allocation table for this device
	fs.devinfo[devIndex].AllocTbl.Shutdown()

	// Shut down the device itself
	fs.devices[devIndex].Close()

	fs.devices[devIndex] = nil
	fs.devinfo[devIndex] = nil
	fs.bcache.UnmountDevice(devIndex)
	fs.itable.UnmountDevice(devIndex)

	return nil
}

func (fs *FileSystem) do_fork(proc *Process) (*Process, error) {
	// Fork a process, duplicating the current root/working directories and
	// all file descriptors.

	child := new(Process)
	fs.procs[fs.pidcounter] = child

	child.pid = fs.pidcounter
	fs.pidcounter++
	child.umask = proc.umask
	child.rootdir = fs.itable.DupInode(proc.rootdir)
	child.workdir = fs.itable.DupInode(proc.workdir)
	child.fs = proc.fs

	child.files = make([]*filp, common.OPEN_MAX)
	for idx, fd := range proc.files {
		if fd != nil {
			child.files[idx] = fd
			fd.file.Dup()
		}
	}

	return child, nil
}

func (fs *FileSystem) do_exit(proc *Process) {
	// Close all open file descriptors
	for i := 0; i < len(proc.files); i++ {
		fd := proc.files[i]
		if fd != nil {
			if err := fd.Close(); err != nil {
				log.Printf("Failed when closing file in exit(%v): %s", proc, err)
			}
		}
		proc.files[i] = nil
	}

	// Return the root/pwd inodes
	fs.itable.PutInode(proc.rootdir)
	fs.itable.PutInode(proc.workdir)
	delete(fs.procs, proc.pid)
}

// Attempt to shut down the file system, only return 'true' if the shutdown
// was successful and the main server loop can exit.
func (fs *FileSystem) do_shutdown() error {
	// Attempt to unmount each non-root device
	for i := common.ROOT_DEVICE + 1; i < common.NR_DEVICES; i++ {
		if fs.devices[i] != nil {
			if fs.itable.IsDeviceBusy(i) {
				return common.EBUSY
			}

			minfo := fs.devinfo[i].MountInfo

			// Clear each inode of the mount info
			minfo.MountPoint.Mounted = nil
			minfo.MountTarget.Mounted = nil

			// Release each inode
			fs.itable.PutInode(minfo.MountPoint)
			fs.itable.PutInode(minfo.MountTarget)

			// Flush and invalidate the cache for the device
			fs.bcache.Flush(i)
			fs.bcache.Invalidate(i)

			// Shut down the alloc table for this device
			fs.devinfo[i].AllocTbl.Shutdown()

			// Shut down the device itself
			fs.devices[i].Close()

			fs.devices[i] = nil
			fs.devinfo[i] = nil
			fs.bcache.UnmountDevice(i)
			fs.itable.UnmountDevice(i)
		}
	}

	// Now try to unmount the root device
	if fs.itable.IsDeviceBusy(common.ROOT_DEVICE) {
		// Cannot unmount this device, so we need to fail
		return common.EBUSY
	} else {
		// Release the inodes for the root process
		proc := fs.procs[common.ROOT_PROCESS]
		if proc != nil { // if it hasn't been shut down already
			if proc.rootdir != proc.workdir && proc.workdir != nil {
				fs.itable.PutInode(proc.workdir)
			}
			fs.itable.PutInode(proc.rootdir)
		}

		fs.bcache.Flush(common.ROOT_DEVICE)
		fs.bcache.Invalidate(common.ROOT_DEVICE)

		fs.devinfo[common.ROOT_DEVICE].AllocTbl.Shutdown()

		fs.devices[common.ROOT_DEVICE].Close()

		fs.devices[common.ROOT_DEVICE] = nil
		fs.devinfo[common.ROOT_DEVICE] = nil
		fs.bcache.UnmountDevice(common.ROOT_DEVICE)
		fs.itable.UnmountDevice(common.ROOT_DEVICE)
	}

	if err := fs.bcache.Shutdown(); err != nil {
		panic(fmt.Sprintf("Failed to shut down block cache: %s", err))
	}
	if err := fs.itable.Shutdown(); err != nil {
		panic(fmt.Sprintf("Failed to shut down block cache: %s", err))
	}

	return nil
}

func (fs *FileSystem) do_chdir(proc *Process, path string) error {
	rip, err := fs.eatPath(proc, path)
	if err != nil {
		return err
	}

	var r error

	if !rip.IsDirectory() {
		r = common.ENOTDIR
	}
	// TODO: Check permissions

	// If error then return inode
	if r != nil {
		fs.itable.PutInode(rip)
		return r
	}

	// Everything is okay, make the change
	fs.itable.PutInode(proc.workdir)
	proc.workdir = rip
	return nil
}

var mode_map = []uint16{
	common.R_BIT,
	common.W_BIT,
	common.R_BIT | common.W_BIT,
	0}

func (fs *FileSystem) do_open(proc *Process, path string, oflags int, omode uint16) (common.Fd, error) {
	// Remap the bottom two bits of oflags
	bits := mode_map[oflags&common.O_ACCMODE]

	var err error
	var rip *common.Inode
	var exist bool = false

	// If O_CREATE is set, try to make the file
	if oflags&common.O_CREAT > 0 {
		// Create a new node by calling new_node()
		omode := common.I_REGULAR | (omode & common.ALL_MODES & proc.umask)
		dirp, newrip, _, err := fs.new_node(proc, path, omode, common.NO_ZONE)
		if err == nil {
			exist = false
		} else if err != common.EEXIST {
			return nil, err
		} else {
			exist = (oflags&common.O_EXCL == 0)
		}

		// we don't need the parent directory
		fs.itable.PutInode(dirp)
		rip = newrip
	} else {
		// grab the inode at the given path
		rip, err = fs.eatPath(proc, path)
		if err != nil {
			return nil, err
		}
	}

	// Find an available filp entry for the file descriptor
	fdindex := -1
	for i := 0; i < len(proc.files); i++ {
		if proc.files[i] == nil {
			fdindex = i
			break
		}
	}

	if fdindex == -1 {
		return nil, common.EMFILE
	}

	err = nil // we'll use this to set error codes

	if exist { // if the file existed already
		// TODO: Check permissions here
		switch rip.Type() {
		case common.I_REGULAR:
			if oflags&common.O_TRUNC > 0 {
				common.Truncate(rip, 0, fs.bcache)
				// Flush the inode so it gets written on next block cache
				fs.itable.FlushInode(rip)
			}
		case common.I_DIRECTORY:
			// Directories cannot be opened in this system
			err = common.EISDIR
		default:
			panic("NYI: Process.Open with non regular/directory")
		}
	}

	if err != nil {
		// Something went wrong, so release the inode
		fs.itable.PutInode(rip)
		return nil, err
	}

	// Make sure there is a 'File' server running
	if rip.File == nil {
		// Spawn a file process to handle reading/writing
		rip.File = file.NewFile(rip)
	}

	// Create a new 'filp' object to expose to the user
	filp := &filp{1, 0, rip.File, rip, bits, new(sync.Mutex)}
	proc.files[fdindex] = filp

	return filp, nil
}

func (fs *FileSystem) do_close(proc *Process, fd common.Fd) error {
	filp, ok := fd.(*filp)
	if !ok {
		return common.EBADF
	}

	// Find this entry in the process table
	for i := 0; i < len(proc.files); i++ {
		if proc.files[i] == filp {
			// This is actually a valid file descriptor
			err := filp.Close()
			proc.files[i] = nil
			return err
		}
	}

	// If we get here, it was not a valid file descriptor
	return common.EBADF
}

func (fs *FileSystem) do_unlink(proc *Process, path string) error {
	// Get the inodes we need to perform the unlink
	dirp, rip, filename, err := fs.unlink_prep(proc, path)
	if err != nil {
		return err
	} else if dirp == nil || rip == nil {
		return common.ENOENT
	}

	// Now test if the call is allowed (altered from Minix)
	if rip.Inum == common.ROOT_INODE {
		err = common.EBUSY
	}

	if err == nil {
		// Perform the unlink
		err = Unlink(dirp, filename)
		rip.Nlinks--
		rip.Dirty = true
	}

	// Regardless, return both inodes
	fs.itable.PutInode(rip)
	fs.itable.PutInode(dirp)
	return err
}

func (fs *FileSystem) do_link(proc *Process, oldpath, newpath string) error {
	// Fetch the file to be linked
	rip, err := fs.eatPath(proc, oldpath)
	if err != nil {
		return err
	}
	// Check if the file has too many links
	if rip.Nlinks >= math.MaxUint16 {
		fs.itable.PutInode(rip)
		return common.EMLINK
	}

	// TODO: only root user can link to directories

	// Grab the new parent directory
	dirp, rest, err := fs.lastDir(proc, newpath)
	if err != nil {
		fs.itable.PutInode(rip)
		return err
	}

	var r error = nil // to help with cleanup

	// Check to see if the target file exists
	newrip, err := fs.advance(proc, dirp, rest)
	if err == nil {
		// The target already exists
		fs.itable.PutInode(newrip)
		r = common.EEXIST
	}

	// Check for links across devices
	if r == nil && rip.Devinfo.Devnum != dirp.Devinfo.Devnum {
		r = common.EXDEV
	}

	// Perform the link operation
	r = Link(dirp, rest, rip.Inum)

	if r == nil { // everything was successful, register the linking
		rip.Nlinks++
		rip.Dirty = true
	}

	// Done, release both inodes
	fs.itable.PutInode(rip)
	fs.itable.PutInode(dirp)
	return r
}

func (fs *FileSystem) do_mkdir(proc *Process, path string, mode uint16) error {
	// Create the new inode. If that fails, return err
	bits := common.I_DIRECTORY | (mode & common.RWX_MODES & proc.umask)
	dirp, rip, rest, err := fs.new_node(proc, path, bits, 0)
	if rip == nil || err == common.EEXIST {
		fs.itable.PutInode(rip)  // can't make dir: it already exists
		fs.itable.PutInode(dirp) // return parent too
		return err
	}

	// Get the inode numbers for . and .. to enter into the directory
	dotdot := dirp.Inum // parent's inode number
	dot := rip.Inum     // inode number of the new dir itself

	// Now make dir entries for . and .. unless the disk is completely full.
	rip.Mode = bits                 // set mode
	err1 := Link(rip, ".", dot)     // enter . in the new dir
	err2 := Link(rip, "..", dotdot) // enter .. in the new dir

	// If both . and .. were entered, increment the link counts

	if err1 == nil && err2 == nil {
		// Normal case
		rip.Nlinks++  // this accounts for .
		dirp.Nlinks++ // this accounts for ..
		dirp.Dirty = true
	} else {
		// It did not work, so remove the new directory
		Unlink(dirp, rest)
		rip.Nlinks--
	}

	// Either way nlinks has been updated
	rip.Dirty = true
	fs.itable.PutInode(dirp)
	fs.itable.PutInode(rip)

	if err1 != nil {
		return err1
	} else if err2 != nil {
		return err2
	}
	return err
}

// Remove a directory from the file system.
func (fs *FileSystem) do_rmdir(proc *Process, path string) error {
	// Get parent/inode and filename
	dirp, rip, filename, err := fs.unlink_prep(proc, path)
	if err != nil {
		return err
	}

	// Check to see if the directory is empty
	if !IsEmpty(rip) {
		return common.ENOTEMPTY
	}

	if path == "." || path == ".." {
		return common.EINVAL
	}
	if rip.Inum == common.ROOT_INODE { // can't remove root
		return common.EBUSY
	}

	// Make sure no one else is using this directory. This is a stronger
	// condition than given in Minix initially, where it just cannot be the
	// root or working directory of a process. Could be relaxed, this is just
	// for sanity.
	if rip.Count > 1 {
		return common.EBUSY
	}

	// Actually try to unlink from the parent
	if err = Unlink(dirp, filename); err != nil {
		return err
	}
	rip.Nlinks--

	// We hold the inodes for both directories, so unlink . and .. from the
	// directory.
	Unlink(rip, "..")
	Unlink(rip, ".")

	rip.Nlinks--
	dirp.Nlinks--

	// If the unlink was possible it has been done, otherwise it has not
	// If unlink was possible, it has been done. Otherwise it has not
	fs.itable.PutInode(rip)
	fs.itable.PutInode(dirp)

	return nil
}
