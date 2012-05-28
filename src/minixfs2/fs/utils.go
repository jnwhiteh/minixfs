package fs

import (
	"math"
	. "minixfs2/common"
)

func (fs *FileSystem) new_node(proc *Process, path string, bits uint16, z0 uint) (*Inode, *Inode, string, error) {
	// Open the parent directory
	dirp, rlast, err := fs.lastDir(proc, path)
	if err != nil {
		return nil, nil, "", err
	}

	if dirp.Nlinks >= math.MaxUint16 {
		fs.itable.PutInode(dirp)
		return nil, nil, "", EMLINK
	}

	// Does the new entry already exist?
	rip, err := fs.advance(proc, dirp, rlast)
	if rip != nil || err == nil {
		// Must exist or something is wrong..
		return nil, nil, "", EEXIST
	}

	// The file/directory does not exist, create it
	var inum int // this is here to fix shadowing of err
	inum, err = rip.Devinfo.AllocTbl.AllocInode()
	if err != nil {
		// Could not allocate new inode
		return nil, nil, "", err
	}
	rip, err = fs.itable.GetInode(rip.Devinfo.Devnum, inum)
	if err != nil {
		// Could not fetch the new inode
		return nil, nil, "", err
	}
	rip.Mode = bits
	rip.Zone[0] = uint32(z0)
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
