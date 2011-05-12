package minixfs

import "os"

// Allocate a new inode, make a directory entry for it on the path 'path' and
// initialise it. If successful, the inode is returned along with a nil error,
// otherwise nil is returned along with the error.

func (fs *FileSystem) new_node(path string, mode uint16, z0 uint) (*Inode, os.Error) {
	// See if the path can be opened down to the last directory
	return nil, os.NewError("NewNode not implemented")
}
