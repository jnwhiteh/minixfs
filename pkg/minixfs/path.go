package minixfs

import (
	"os"
	"path/filepath"
	"strings"
)

var ENOTDIR = os.NewError("ENOTDIR: not a directory")
var ENOENT = os.NewError("ENOENT: no such file or directory")

// EathPath parses the path 'path' and retrieves the associated inode.
func (fs *FileSystem) eat_path(proc *Process, path string) *Inode {
	ldip, rest, err := fs.last_dir(proc, path)
	if err != nil {
		return nil // could not open final directory
	}

	// If there is no more path to go, return
	if len(rest) == 0 {
		return ldip
	}

	// Get final compoennt of the path
	rip := fs.advance(proc, ldip, rest)
	fs.put_inode(ldip)
	return rip
}

// LastDir parses 'path' as far as the last directory, fetching the inode and
// returning it along with the final portion of the path and any error that
// might have occurred.
func (fs *FileSystem) last_dir(proc *Process, path string) (*Inode, string, os.Error) {
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

	fs.dup_inode(rip) // inode will be returned with put_inode

	var pathlist []string
	if filepath.IsAbs(path) {
		pathlist = strings.Split(path, filepath.SeparatorString, -1)
		pathlist = pathlist[1:]
	} else {
		pathlist = strings.Split(path, filepath.SeparatorString, -1)
	}

	for i := 0; i < len(pathlist)-1; i++ {
		newip := fs.advance(proc, rip, pathlist[i])
		fs.put_inode(rip)
		if newip == nil {
			return nil, "", ENOENT
		}
		rip = newip
	}

	return rip, pathlist[len(pathlist)-1], nil
}

// Advance looks up the component 'path' in the directory 'dirp', returning
// the inode.

func (fs *FileSystem) advance(proc *Process, dirp *Inode, path string) *Inode {
	// if there is no path, just return this inode
	if len(path) == 0 {
		rip, _ := fs.get_inode(dirp.Inum())
		return rip
	}

	// check for a nil inode
	if dirp == nil {
		return nil
	}

	// check to ensure that this inode is a directory
	if dirp.GetType() != I_DIRECTORY {
		return nil
	}

	// TODO: Is there a way to signal an error here?
	// ensure that 'path' is an entry in the directory
	numb, err := fs.search_dir(dirp, path)
	if err != nil {
		return nil
	}

	// don't go beyond the current root directory, ever
	if dirp == proc.rootdir && path == ".." {
		rip, _ := fs.get_inode(dirp.Inum())
		return rip
	}

	// the component has been found in the directory, get the inode
	rip, _ := fs.get_inode(uint(numb))
	if rip == nil {
		return nil
	}

	// TODO: Handle mounted file systems here

	return rip
}

// SearchDir searches for an entry named 'path' in the directory given by
// 'dirp'. This function differs from the minix version.
func (fs *FileSystem) search_dir(dirp *Inode, path string) (int, os.Error) {
	if dirp.GetType() != I_DIRECTORY {
		return 0, ENOTDIR
	}

	// step through the directory on block at a time
	numEntries := dirp.Size / DIR_ENTRY_SIZE
	for pos := 0; pos < int(dirp.Size); pos += int(fs.super.Block_size) {
		b := fs.read_map(dirp, uint(pos)) // get block number
		bp := fs.get_block(int(b), DIRECTORY_BLOCK)
		if bp == nil {
			panic("get_block returned NO_BLOCK")
		}

		dirarr := bp.block.(DirectoryBlock)
		for i := 0; i < len(dirarr) && numEntries > 0; i++ {
			if dirarr[i].HasName(path) {
				return int(dirarr[i].Inum), nil
			}
			numEntries--
		}
	}

	return 0, ENOENT
}
