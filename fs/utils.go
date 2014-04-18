package fs

import (
	"math"
	"github.com/jnwhiteh/minixfs/common"
)

func (fs *FileSystem) new_node(proc *Process, path string, bits uint16, z0 uint) (*common.Inode, *common.Inode, string, error) {
	// Open the parent directory
	dirp, rlast, err := fs.lastDir(proc, path)
	if err != nil {
		return nil, nil, "", err
	}

	if dirp.Nlinks >= math.MaxUint16 {
		fs.itable.PutInode(dirp)
		return nil, nil, "", common.EMLINK
	}

	// Does the new entry already exist?
	rip, err := fs.advance(proc, dirp, rlast)
	if rip != nil || err == nil {
		// Must exist or something is wrong..
		return nil, nil, "", common.EEXIST
	}

	// The file/directory does not exist, create it
	devinfo := dirp.Devinfo
	var inum int // this is here to fix shadowing of err
	inum, err = devinfo.AllocTbl.AllocInode()
	if err != nil {
		// Could not allocate new inode
		return nil, nil, "", err
	}
	rip, err = fs.itable.GetInode(devinfo.Devnum, inum)
	if err != nil {
		// Could not fetch the new inode
		return nil, nil, "", err
	}
	rip.Mode = bits
	rip.Zone[0] = uint32(z0)
	rip.Size = 0
	// TODO: Add uid/gid here
	rip.Nlinks++

	// Force the inode to disk before making a directory entry to make the
	// system more robust in the face of a crash: an inode with no
	// directory entry is much better than the opposite.
	fs.itable.FlushInode(rip)

	// New inode acquired. Try to make directory entry.
	err = Link(dirp, rlast, inum)
	if err != nil {
		fs.itable.PutInode(dirp)
		rip.Nlinks-- // pity, have to free disk inode
		rip.Dirty = true // dirty inodes are written out
		fs.itable.PutInode(rip) // this call will free the inode
		return nil, nil, "", err
	}

	// Return the parent directory and the new inode
	return dirp, rip, rlast, nil
}

// Given a path, fetch the inode for the parent directory of final entry and
// the inode of the final entry itself. In addition, return the portion of the
// path that is the filename of the final entry, so it can be removed from the
// parent directory, and any error that may have occurred.
func (fs *FileSystem) unlink_prep(proc *Process, path string) (*common.Inode, *common.Inode, string, error) {
	// Get the last directory in the path
	dirp, rest, err := fs.lastDir(proc, path)
	if dirp == nil {
		return nil, nil, "", err
	}

	// The last directory exists. Does the file also exist?
	rip, err := fs.advance(proc, dirp, rest)
	if rip == nil || err != nil {
		fs.itable.PutInode(dirp)
		return nil, nil, "", err
	}

	// Do not remove a mount point
	if rip.Inum == common.ROOT_INODE {
		fs.itable.PutInode(dirp)
		fs.itable.PutInode(rip)
		return nil, nil, "", common.EBUSY
	}

	return dirp, rip, rest, nil
}
