package inode

import (
	. "minixfs/common"
)

func Lookup(rip *Inode, name string) (bool, int, int) {
	if !rip.IsDirectory() {
		return false, NO_DEV, NO_INODE
	}

	inum := 0
	err := search_dir(rip, name, &inum, LOOKUP)
	if err != nil {
		return false, NO_DEV, NO_INODE
	}

	return true, rip.Devnum, inum
}


func Link(rip *Inode, name string, inum int) error {
	if !rip.IsDirectory() {
		return ENOTDIR
	}

	// Add the entry to the directory
	err := search_dir(rip, name, &inum, ENTER)
	return err
}

func Unlink(rip *Inode, name string) error {
	if !rip.IsDirectory() {
		return ENOTDIR
	}

	inum := 0
	err := search_dir(rip, name, &inum, DELETE)
	return err
}

func IsEmpty(rip *Inode) bool {
	if !rip.IsDirectory() {
		return false
	}

	zeroinode := 0
	if err := search_dir(rip, "", &zeroinode, IS_EMPTY); err != nil {
		return false
	}
	return true
}
