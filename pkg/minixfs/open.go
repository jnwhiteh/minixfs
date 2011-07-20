package minixfs

import "os"

// Allocate a new inode, make a directory entry for it on the path 'path' and
// initialise it. If successful, the inode is returned along with a nil error,
// otherwise nil is returned along with the error.
func (fs *FileSystem) new_node(proc *Process, path string, bits uint16, z0 uint) (*Inode, os.Error) {
	var err os.Error

	// See if the path can be opened down to the last directory
	dirp, rlast, err := fs.last_dir(proc, path)
	if err != nil {
		return nil, err
	}

	// The final directory is accessible. Get the final component of the path
	rip, err := fs.advance(proc, dirp, rlast)
	if rip == nil && err == ENOENT {

		// Last component does not exist. Make new directory entry
		rip = fs.alloc_inode(dirp.dev, bits)
		if rip == nil {
			// Can't create new inode, out of inodes
			fs.put_inode(dirp)
			return nil, nil
		}

		// Force the inode to disk before making a directory entry to make the
		// system more robust in the face of a crash: an inode with no
		// directory entry is much better than the opposite.
		rip.IncNlinks()
		rip.SetZone(0, uint32(z0))
		fs.icache.WriteInode(rip)

		// New inode acquired. Try to make directory entry.
		inum := int(rip.inum)
		err = fs.search_dir(dirp, rlast, &inum, ENTER)
		if err != nil {
			fs.put_inode(dirp)
			rip.DecNlinks()    // pity, have to free disk inode
			rip.SetDirty(true) // dirty inodes are written out
			fs.put_inode(rip)  // this call frees the inode
			return nil, err
		}
	} else {
		// Either last component exists or there is some problem
		if rip != nil {
			err = EEXIST
		}
	}

	// Return the last directory inode and exit
	fs.put_inode(dirp)
	return rip, err
}
