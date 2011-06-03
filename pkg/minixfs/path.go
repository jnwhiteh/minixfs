package minixfs

import (
	"os"
	"path/filepath"
	"strings"
)

// EathPath parses the path 'path' and retrieves the associated inode.
func (fs *FileSystem) eat_path(proc *Process, path string) (*Inode, os.Error) {
	ldip, rest, err := fs.last_dir(proc, path)
	if err != nil {
		return nil, err // could not open final directory
	}

	// If there is no more path to go, return
	if len(rest) == 0 {
		return ldip, nil
	}

	// Get final compoennt of the path
	rip, _ := fs.advance(proc, ldip, rest)
	fs.put_inode(ldip)
	return rip, nil
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
		newip, _ := fs.advance(proc, rip, pathlist[i])
		fs.put_inode(rip)
		if newip == nil {
			return nil, "", ENOENT
		}
		rip = newip
	}

	if rip.GetType() != I_DIRECTORY {
		// last file of path prefix is not a directory
		fs.put_inode(rip)
		return nil, "", ENOTDIR
	}

	return rip, pathlist[len(pathlist)-1], nil
}

// Advance looks up the component 'path' in the directory 'dirp', returning
// the inode.
func (fs *FileSystem) advance(proc *Process, dirp *Inode, path string) (*Inode, os.Error) {
	// if there is no path, just return this inode
	if len(path) == 0 {
		return fs.get_inode(dirp.dev, dirp.inum)
	}

	// check for a nil inode
	if dirp == nil {
		return nil, nil // TODO: This should return something
	}

	// If 'path' is not present in the directory, signal error
	var numb int
	err := fs.search_dir(dirp, path, &numb, LOOK_UP)
	if err != nil {
		return nil, err
	}

	// don't go beyond the current root directory, ever
	if dirp == proc.rootdir && path == ".." {
		return fs.get_inode(dirp.dev, dirp.inum)
	}

	// the component has been found in the directory, get the inode
	rip, _ := fs.get_inode(dirp.dev, uint(numb))
	if rip == nil {
		return nil, nil // TODO: What error should we return here?
	}

	if rip.inum == ROOT_INODE {
		if dirp.inum == ROOT_INODE {
			// TODO: What does this do?
			if path[1] == '.' {
				if fs.devs[rip.dev] != nil {
					// we can skip the superblock search here since we know
					// that 'i' is the device that we're looking at.
					sp := fs.supers[rip.dev]
					fs.put_inode(rip)
					mnt_dev := sp.imount.dev
					inumb := sp.imount.inum
					rip2, _ := fs.get_inode(mnt_dev, inumb) // TODO: ignore error
					rip, _ = fs.advance(proc, rip2, path)
					fs.put_inode(rip2)
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
	for rip != nil && rip.mount {
		// The inode is indeed mounted on
		for i := 0; i < NR_SUPERS; i++ {
			if fs.supers[i] != nil && fs.supers[i].imount == rip {
				// Release the inode mounted on. Replace by the inode of the
				// root inode of the mounted device.
				fs.put_inode(rip)
				rip, _ = fs.get_inode(i, ROOT_INODE) // TODO: ignore error
				break
			}
		}
	}
	return rip, nil
}

type searchDirFlag int

const (
	ENTER    = iota
	DELETE   = iota
	LOOK_UP  = iota
	IS_EMPTY = iota
)

// SearchDir searches for an entry named 'path' in the directory given by
// 'dirp'. The behaviour of the function changes depending on the given flags:
//
// if flag == ENTER enter 'path' into the directory with inode # numb
// if flag == DELETE delete 'path' from the directory
// if flag == LOOK_UP search for 'path' and return inode # in 'numb'
// if flag == IS_EMPTY return OK if only . and .. in dir else ENOTEMPTY
func (fs *FileSystem) search_dir(dirp *Inode, path string, numb *int, flag searchDirFlag) os.Error {
	// If dirp is not a pointer to a directory node, error
	if dirp.GetType() != I_DIRECTORY {
		return ENOTDIR
	}

	// TODO: Check permissions (see minix source)

	super := fs.supers[dirp.dev]

	// step through the directory on block at a time
	var bp *buf
	var dp *disk_dirent
	old_slots := int(dirp.Size / DIR_ENTRY_SIZE)
	new_slots := 0
	e_hit := false
	match := false
	extended := false

	for pos := 0; pos < int(dirp.Size); pos += int(super.Block_size) {
		b := fs.read_map(dirp, uint(pos)) // get block number
		bp = fs.get_block(dirp.dev, int(b), DIRECTORY_BLOCK, NORMAL)
		if bp == nil {
			panic("get_block returned NO_BLOCK")
		}

		// Search the directory block
		dirarr := bp.block.(DirectoryBlock)
		for i := 0; i < len(dirarr); i++ {
			dp = &dirarr[i]

			new_slots++
			if new_slots > old_slots { // not found, but room left
				if flag == ENTER {
					e_hit = true
					break
				}
			}

			// Match occurs if string found
			if flag != ENTER && dp.Inum != 0 {
				if flag == IS_EMPTY {
					// If this succeeds, dir is not empty
					if !dp.HasName(".") && !dp.HasName("..") {
						match = true
					}
				} else {
					if dp.HasName(path) {
						match = true
					}
				}
			}

			if match {
				var r os.Error = nil
				// LOOK_UP or DELETE found what it wanted
				if flag == IS_EMPTY {
					r = ENOTEMPTY
				} else if flag == DELETE {
					// TODO: Save inode for recovery
					dp.Inum = 0 // erase entry
					bp.dirty = true
					dirp.dirty = true
				} else {
					*numb = int(dp.Inum)
				}
				fs.put_block(bp, DIRECTORY_BLOCK)
				return r
			}

			// Check for free slot for the benefit of ENTER
			if flag == ENTER && dp.Inum == 0 {
				e_hit = true // we found a free slot
				break
			}
		}

		// The whole block has been searched or ENTER has a free slot
		if e_hit { // e_hit set if ENTER can be performed now
			break
		}
		fs.put_block(bp, DIRECTORY_BLOCK) // otherwise continue searching dir
	}

	// The whole directory has now been searched
	if flag != ENTER {
		if flag == IS_EMPTY {
			return nil
		} else {
			return ENOENT
		}
	}

	// This call is for ENTER. If no free slot has been found so far, try to
	// extend directory.
	if !e_hit { // directory is full and no room left in last block
		new_slots++ // increase directory size by 1 entry
		// TODO: Does this rely on overflow? Does it work?
		if new_slots == 0 { // dir size limited by slot count (overflow?)
			return EFBIG
		}
		var err os.Error
		bp, err = fs.new_block(dirp, uint(dirp.Size), DIRECTORY_BLOCK)
		if err != nil {
			return err
		}
		dirarr := bp.block.(DirectoryBlock)
		dp = &dirarr[0]
		extended = true
	}

	// 'bp' now points to a directory block with space. 'dp' points to a slot
	// in that block.

	// Set the name of this directory entry
	pathb := []byte(path)
	if len(pathb) < NAME_MAX {
		dp.Name[len(pathb)] = 0
	}
	for i := 0; i < NAME_MAX && i < len(pathb); i++ {
		dp.Name[i] = pathb[i]
	}
	dp.Inum = uint32(*numb)
	bp.dirty = true
	fs.put_block(bp, DIRECTORY_BLOCK)
	// TODO: update times
	dirp.dirty = true
	if new_slots > old_slots {
		dirp.Size = int32(new_slots * DIR_ENTRY_SIZE)
		// Send the change to disk if the directory is extended
		if extended {
			fs.icache.WriteInode(dirp)
		}
	}
	return nil
}
