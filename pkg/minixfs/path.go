package minixfs

import "os"
import "path"

// Given a path, parse it as far as the last directory and fetch the inode
// for the last directory and return it along with the final portion of the
// path and any error that might have occurred.

func (fs *FileSystem) LastDir(filename string) (*Inode, string, os.Error) {
	filename = path.Clean(filename)

	var rip *Inode
	if path.IsAbs(filename) {
		rip = fs.RootDir
	} else {
		rip = fs.WorkDir
	}

	// If directory has been removed or path is empty, return ENOENT
	if rip.Nlinks == 0 || len(filename) == 0 {
		return nil, "", os.NewError("ENOENT: no such file or directory")
	}

	fs.DupInode(rip) // tell the fs that we're using the inode


	return nil, "", os.NewError("LastDir not implemented")
}

