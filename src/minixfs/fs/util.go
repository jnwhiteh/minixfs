package fs

import (
	"log"
	"math"
	. "minixfs/common"
	"minixfs/inode"
)

func (fs *FileSystem) wipeInode(rip LockedInode) {
	// NYI
}

// Unmount a device (must be called under the fs.m.device mutex)
func (fs *FileSystem) unmount(devno int) error {
	if fs.devices[devno] == nil {
		return EINVAL
	}

	// See if the mounted device is busy. Only one inode using it should be
	// open, the root inode, and only once.
	if fs.icache.IsDeviceBusy(devno) {
		return EBUSY // can't unmount a busy file system
	}

	minfo := fs.mountinfo[devno]
	if minfo.imount != nil {
		rip := fs.icache.RLockInode(minfo.imount)
		wrip := fs.icache.WLockInode(rip)
		wrip.SetMountPoint(false)
		fs.icache.PutInode(wrip) // release the inode mounted on
	}
	if minfo.isup != nil {
		rip := fs.icache.RLockInode(minfo.isup)
		fs.icache.PutInode(rip) // release the root inode of the mounted fs
	}

	// Flush and invalidate the cache
	fs.bcache.Flush(devno)
	fs.bcache.Invalidate(devno)

	// Close the device the file system lives on
	if err := fs.devices[devno].Close(); err != nil {
		log.Printf("Error when closing device %d: %s", devno, err)
	}

	// Shut down the bitmap process
	if err := fs.bitmaps[devno].Close(); err != nil {
		log.Printf("Error when closing bitmap %d: %s", devno, err)
	}

	fs.devices[devno] = nil
	fs.bitmaps[devno] = nil
	fs.devinfo[devno] = NO_DEVINFO
	return nil
}

func (fs *FileSystem) close(proc *Process, file *File) error {
	if file.fd == NO_FILE {
		return EBADF
	}

	fs.icache.PutInode(fs.icache.RLockInode(file.inode))
	proc.filp[file.fd] = nil
	proc.files[file.fd] = nil

	file.Filp.count--
	if file.Filp.count == 0 {
		fs.filps[file.fd] = nil
	}
	file.fd = NO_FILE
	return nil
}

// Allocate a new inode, making a directory entry for it on the path 'path. If
// successful, the parent directory is returned, along with the new node
// itself, and an nil error.
func (fs *FileSystem) newNode(proc *Process, path string, bits uint16, z0 uint) (LockedInode, LockedInode, string, error) {
	// See if the path can be opened down to the last directory
	dirp, rlast, err := fs.lastDir(proc, path)
	if err != nil {
		return nil, nil, "", err
	}

	if dirp.Links() >= math.MaxUint16 {
		fs.icache.PutInode(dirp)
		return nil, nil, "", EMLINK
	}

	wdirp := fs.icache.WLockInode(dirp)

	// The final directory is accessible. Get the final component of the path
	rip, err := fs.advance(proc, dirp, rlast)
	var wrip LockedInode

	if rip == nil && err == ENOENT {
		// Last component does not exist. Make new directory entry
		var inum int // this is here to fix shadowing of err
		inum, err = fs.bitmaps[dirp.Devnum()].AllocInode()
		// TODO: Get the current uid/gid
		rip, err = fs.icache.GetInode(dirp.Devnum(), inum)
		if rip == nil {
			// Can't create new inode, out of inodes
			fs.icache.PutInode(dirp)
			return nil, nil, "", ENFILE
		}
		wrip = fs.icache.WLockInode(rip)
		wrip.SetMode(bits)
		wrip.SetZone(0, uint32(z0))
		wrip.IncLinks()

		// Force the inode to disk before making a directory entry to make the
		// system more robust in the face of a crash: an inode with no
		// directory entry is much better than the opposite.
		fs.icache.FlushInode(wrip)

		// New inode acquired. Try to make directory entry.
		err = inode.Link(wdirp, rlast, inum)

		if err != nil {
			fs.icache.PutInode(dirp)
			wrip.DecLinks()         // pity, have to free disk inode
			wrip.SetDirty(true)     // dirty inodes are written out
			fs.icache.PutInode(rip) // this call frees the inode
			return nil, nil, "", err
		}
	} else {
		// Either last component exists or there is some problem
		if rip != nil {
			err = EEXIST
		}
	}

	// We now return the parent directory inode, so don't put it here
	return wdirp, wrip, rlast, err
}

// Given a path, fetch the inode for the parent directory of final entry and
// the inode of the final entry itself. In addition, return the portion of the
// path that is the filename of the final entry, so it can be removed from the
// parent directory, and any error that may have occurred.
func (fs *FileSystem) unlinkPrep(proc *Process, path string) (LockedInode, LockedInode, string, error) {
	// Get the last directory in the path
	rldirp, rest, err := fs.lastDir(proc, path)
	if rldirp == nil {
		return nil, nil, "", err
	}

	// The last directory exists. Does the file also exist?
	rip, err := fs.advance(proc, rldirp, rest)
	if rip == nil || err != nil {
		fs.icache.PutInode(rldirp)
		return nil, nil, "", err
	}

	// Do not remove a mount point
	if rip.Inum() == ROOT_INODE {
		fs.icache.PutInode(rldirp)
		fs.icache.PutInode(rip)
		return nil, nil, "", EBUSY
	}

	wrldirp := fs.icache.WLockInode(rldirp)
	wrip := fs.icache.WLockInode(rip)

	return wrldirp, wrip, rest, nil
}

// Unlink a the file 'rip' from 'dirp', with filename 'filename'. The altered
// inodes are not 'put', so that must be done by the caller.
func (fs *FileSystem) unlinkFile(dirp, rip LockedInode, filename string) error {
	var err error

	// if rip is not nil, it is used to get access to the inode
	if rip == nil {
		// Search for file in directory and try to get its inode
		log.Printf("Looking for entry %v in %v", filename, dirp.Inum())
		if ok, dnum, inum := inode.Lookup(dirp, filename); ok {
			var rrip Inode
			rrip, err = fs.icache.GetInode(dnum, inum)
			rip = fs.icache.WLockInode(rrip)
		} else {
			err = ENOENT
		}

		if err != nil {
			return ENOENT
		}
	}

	err = inode.Unlink(dirp, filename)
	if err == nil {
		rip.DecLinks()
		// TODO: Update times
		rip.SetDirty(true)
	}

	return err
}
