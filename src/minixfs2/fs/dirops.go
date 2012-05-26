package fs

import (
	. "minixfs2/common"
)

func Lookup(rip *Inode, name string) (bool, int, int) {
	if !rip.IsDirectory() {
		return false, NO_DEV, NO_INODE
	}

	dirp := rip

	inum := 0
	err := search_dir(dirp, name, &inum, LOOKUP)
	if err != nil {
		return false, NO_DEV, NO_INODE
	}

	return true, dirp.Devinfo.Devnum, inum
}

func Link(rip *Inode, name string, inum int) error {
	if !rip.IsDirectory() {
		return ENOTDIR
	}

	dirp := rip

	// Add the entry to the directory
	err := search_dir(dirp, name, &inum, ENTER)
	return err
}

func Unlink(rip *Inode, name string) error {
	if !rip.IsDirectory() {
		return ENOTDIR
	}

	dirp := rip

	inum := 0
	err := search_dir(dirp, name, &inum, DELETE)
	return err
}

func IsEmpty(rip *Inode) bool {
	if !rip.IsDirectory() {
		return false
	}

	dirp := rip

	zeroinode := 0
	if err := search_dir(dirp, "", &zeroinode, IS_EMPTY); err != nil {
		return false
	}
	return true
}
