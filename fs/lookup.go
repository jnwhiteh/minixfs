package fs

import (
	. "minixfs/common"
	"path/filepath"
	"strings"
)

func (fs *FileSystem) eatPath(proc *Process, path string) (*Inode, error) {
	ldip, rest, err := fs.lastDir(proc, path)
	if err != nil {
		return nil, err // could not open final directory
	}

	// If there is no more path to go, return
	if len(rest) == 0 {
		return ldip, nil
	}

	// Get final component of the path
	rip, err := fs.advance(proc, ldip, rest)
	fs.itable.PutInode(ldip)
	return rip, err
}

func (fs *FileSystem) lastDir(proc *Process, path string) (*Inode, string, error) {
	path = filepath.Clean(path)

	var rip *Inode
	if filepath.IsAbs(path) {
		rip = proc.rootdir
	} else {
		rip = proc.workdir
	}

	// If directory has been removed or path is empty, return ENOENT
	if rip.Nlinks == 0 || len(path) == 0 {
		return nil, "", ENOENT
	}

	// We're going to use this inode, so make a copy of it
	rip = fs.itable.DupInode(rip)

	pathlist := strings.Split(path, string(filepath.Separator))
	if filepath.IsAbs(path) {
		pathlist = pathlist[1:]
	}

	// Scan the path component by component
	for i := 0; i < len(pathlist)-1; i++ {
		// Fetch the next component in the path
		newrip, err := fs.advance(proc, rip, pathlist[i])

		// Current inode obsolete or irrelevant
		fs.itable.PutInode(rip)
		if newrip == nil || err != nil {
			return nil, "", ENOENT
		}
		// Continue to the next component
		rip = newrip
	}

	if rip.Type() != I_DIRECTORY {
		// The penultimate path entry was not a directory, so return nil
		fs.itable.PutInode(rip)
		return nil, "", ENOTDIR
	}

	return rip, pathlist[len(pathlist)-1], nil
}

func (fs *FileSystem) advance(proc *Process, dirp *Inode, path string) (*Inode, error) {
	// if there is no path, just return this inode
	if len(path) == 0 {
		return fs.itable.DupInode(dirp), nil
	}

	// check for a nil inode
	if dirp == nil {
		return nil, ENOENT
	}

	// don't go beyond the current root directory, ever
	if dirp == proc.rootdir && path == ".." {
		return fs.itable.DupInode(dirp), nil
	}

	// If 'path' is not present in the directory, signal error
	var rip *Inode
	var err error

	if ok, dnum, inum := Lookup(dirp, path); ok {
		rip, err = fs.itable.GetInode(dnum, inum)
	} else {
		err = ENOENT
	}

	if err != nil {
		return nil, ENOENT
	}

	if rip.Inum == ROOT_INODE {
		if dirp.Inum == ROOT_INODE {
			// TODO: What does this do?
			if path[1] == '.' {
				panic("weird case in lookup, whata is this?")
//				if fs.devices[devnum] != nil {
//					// we can skip the superblock search here since we know
//					// that 'i' is the device that we're looking at.
//					mountinfo := fs.mountinfo[devnum]
//					fs.itable.PutInode(rip)
//					mnt_dev := mountinfo.imount.Devnum()
//					inumb := mountinfo.imount.Inum()
//					rip2, _ := fs.itable.GetInode(mnt_dev, inumb) // TODO: ignore error
//					rip, _ = fs.advance(proc, rip2, path)
//					fs.itable.PutInode(rip2)
//				}
			}
		}
	}

	if rip == nil {
		return nil, nil // TODO: Error here?
	}

	// See if the inode is mounted on. If so, switch to the root directory of
	// the mounted file system. The super_block provides the linkage between
	// the inode mounted on and the root directory of the mounted file system.
	// TODO: MOUNTING RIGHT NOW IS NOT VERY ROBUST
	if rip != nil && rip.Mounted != nil {
		// The inode is indeed mounted on
		// Release the inode that is mounted on and replace it with the root
		// inode of the mounted device
		minfo := rip.Mounted
		fs.itable.PutInode(rip)
		rip = fs.itable.DupInode(minfo.MountTarget)
	}
	return rip, nil
}
