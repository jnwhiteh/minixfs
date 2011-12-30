package fs

import (
	. "../../minixfs/common/_obj/minixfs/common"
	"log"
	"os"
)

// Unmount a device (must be called under the fs.m.device mutex)
func (fs *FileSystem) unmount(devno int) os.Error {
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
		minfo.imount.Mount = false       // inode returns to normal
		fs.icache.PutInode(minfo.imount) // release the inode mounted on
	}
	if minfo.isup != nil {
		fs.icache.PutInode(minfo.isup) // release the root inode of the mounted fs
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

func (fs *FileSystem) close(proc *Process, file *File) os.Error {
	if file.fd == NO_FILE {
		return EBADF
	}

	fs.icache.PutInode(file.inode)
	proc.filp[file.fd] = nil
	proc.files[file.fd] = nil

	file.Filp.count--
	if file.Filp.count == 0 {
		fs.filps[file.fd] = nil
	}
	file.fd = NO_FILE
	return nil
}

// Allocate a new inode, make a directory entry for it on the path 'path' and
// initialise it. If successful, the inode is returned along with a nil error,
// otherwise nil is returned along with the error.
func (fs *FileSystem) newNode(proc *Process, path string, bits uint16, z0 uint) (*CacheInode, os.Error) {
	var err os.Error

	// See if the path can be opened down to the last directory
	dirp, rlast, err := fs.lastDir(proc, path)
	if err != nil {
		return nil, err
	}

	// The final directory is accessible. Get the final component of the path
	rip, err := fs.advance(proc, dirp, rlast)
	if rip == nil && err == ENOENT {

		// Last component does not exist. Make new directory entry
		inum, err := dirp.Bitmap.AllocInode(bits)
		rip, err = fs.icache.GetInode(dirp.Devno, inum)

		if rip == nil {
			// Can't create new inode, out of inodes
			fs.icache.PutInode(dirp)
			return nil, nil
		}

		// Force the inode to disk before making a directory entry to make the
		// system more robust in the face of a crash: an inode with no
		// directory entry is much better than the opposite.
		rip.Inode.Nlinks++
		rip.Inode.Zone[0] = uint32(z0)
		fs.icache.FlushInode(rip)

		// New inode acquired. Try to make directory entry.
		dinode := dirp.Dinode()
		err = dinode.Link(rlast, inum)
		if err != nil {
			fs.icache.PutInode(dirp)
			rip.Inode.Nlinks--      // pity, have to free disk inode
			rip.Dirty = true        // dirty inodes are written out
			fs.icache.PutInode(rip) // this call frees the inode
			return nil, err
		}
	} else {
		// Either last component exists or there is some problem
		if rip != nil {
			err = EEXIST
		}
	}

	// Return the last directory inode and exit
	fs.icache.PutInode(dirp)
	return rip, err
}

func (fs *FileSystem) eatPath(proc *Process, path string) (*CacheInode, os.Error) {
	// NYI
	return nil, nil
}

func (fs *FileSystem) wipeInode(rip *CacheInode) {
	// NYI
}

func (fs *FileSystem) lastDir(proc *Process, path string) (*CacheInode, string, os.Error) {
	return nil, "", nil
}

func (fs *FileSystem) advance(proc *Process, rip *CacheInode, path string) (*CacheInode, os.Error) {
	return nil, nil
}
