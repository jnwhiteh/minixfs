package fs

import (
	. "minixfs/common"
	"minixfs/inode"
	"path/filepath"
	"strings"
)

func (fs *FileSystem) eatPath(proc *Process, path string) (Inode, error) {
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
	fs.icache.PutInode(ldip)
	return rip, err
}

func (fs *FileSystem) lastDir(proc *Process, path string) (Inode, string, error) {
	path = filepath.Clean(path)

	var rip Inode
	if filepath.IsAbs(path) {
		rip = fs.icache.RLockInode(proc.rootdir)
	} else {
		rip = fs.icache.RLockInode(proc.workdir)
	}

	// If directory has been removed or path is empty, return ENOENT
	if rip.Links() == 0 || len(path) == 0 {
		return nil, "", ENOENT
	}

	// We're going to use this inode, so make a copy of it (read)
	rip = fs.icache.DupInode(rip)

	var pathlist []string
	if filepath.IsAbs(path) {
		pathlist = strings.Split(path, string(filepath.Separator))
		pathlist = pathlist[1:]
	} else {
		pathlist = strings.Split(path, string(filepath.Separator))
	}

	for i := 0; i < len(pathlist)-1; i++ {
		// Fetch the next component in the path
		newrip, err := fs.advance(proc, rip, pathlist[i])

		// Regardless of whether it was there or not, we're done with the
		// current path level, so return that to the cache
		fs.icache.PutInode(rip)
		if newrip == nil || err != nil {
			return nil, "", ENOENT
		}
		// The new inode is already locked (type Indode) so we don't have to
		// do anything special.
		rip = newrip
	}

	if rip.Type() != I_DIRECTORY {
		// The penultimate path entry was not a directory, so return nil
		fs.icache.PutInode(rip)
		return nil, "", ENOTDIR
	}

	return rip, pathlist[len(pathlist)-1], nil
}

func (fs *FileSystem) advance(proc *Process, dirp Inode, path string) (Inode, error) {
	// if there is no path, just return this inode
	if len(path) == 0 {
		return fs.icache.DupInode(dirp), nil
	}

	// check for a nil inode
	if dirp == nil {
		return nil, ENOENT
	}

	// don't go beyond the current root directory, ever
	if dirp == proc.rootdir && path == ".." {
		return fs.icache.DupInode(dirp), nil
	}

	// If 'path' is not present in the directory, signal error
	var rip Inode
	var err error

	if ok, dnum, inum := inode.Lookup(dirp, path); ok {
		rip, err = fs.icache.GetInode(dnum, inum)
	} else {
		err = ENOENT
	}

	if err != nil {
		return nil, ENOENT
	}

	if rip.Inum() == ROOT_INODE {
		if dirp.Inum() == ROOT_INODE {
			// TODO: What does this do?
			if path[1] == '.' {
				if fs.devices[rip.Devnum()] != nil {
					// we can skip the superblock search here since we know
					// that 'i' is the device that we're looking at.
					mountinfo := fs.mountinfo[rip.Devnum()]
					fs.icache.PutInode(rip)
					mnt_dev := mountinfo.imount.Devnum()
					inumb := mountinfo.imount.Inum()
					rip2, _ := fs.icache.GetInode(mnt_dev, inumb) // TODO: ignore error
					rip, _ = fs.advance(proc, rip2, path)
					fs.icache.PutInode(rip2)
				}
			}
		}
	}

	if rip == nil {
		return nil, nil // TODO: Error here?
	}

	// See if the inode is mounted on. If so, switch to the root directory of
	// the mounted file system. The super_block provides the linkage between
	// the inode mounted on and the root directory of the mounted file system.
	for rip != nil && rip.MountPoint() {
		// The inode is indeed mounted on
		for i := 0; i < NR_DEVICES; i++ {
			if fs.mountinfo[i].imount == rip {
				// Release the inode mounted on. Replace by the inode of the
				// root inode of the mounted device.
				fs.icache.PutInode(rip)
				rip, _ = fs.icache.GetInode(i, ROOT_INODE) // TODO: ignore error
				break
			}
		}
	}
	return rip, nil
}
