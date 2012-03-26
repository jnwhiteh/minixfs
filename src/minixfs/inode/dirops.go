package inode

import (
	"log"
	. "minixfs/common"
)

func Lookup(rip Inode, name string) (bool, int, int) {
	if !rip.IsDirectory() {
		log.Printf("this happens: %v, %v, %b", rip.Inum(), name, rip.GetMode())
		return false, NO_DEV, NO_INODE
	}

	dirp := rip.(*cacheInode)

	inum := 0
	err := search_dir(dirp, name, &inum, LOOKUP)
	if err != nil {
		return false, NO_DEV, NO_INODE
	}

	return true, dirp.devnum, inum
}

func Link(rip LockedInode, name string, inum int) error {
	if !rip.IsDirectory() {
		return ENOTDIR
	}

	dirp := rip.(*cacheInode)

	// Add the entry to the directory
	err := search_dir(dirp, name, &inum, ENTER)
	return err
}

func Unlink(rip LockedInode, name string) error {
	if !rip.IsDirectory() {
		return ENOTDIR
	}

	dirp := rip.(*cacheInode)

	inum := 0
	err := search_dir(dirp, name, &inum, DELETE)
	return err
}

func IsEmpty(rip Inode) bool {
	if !rip.IsDirectory() {
		return false
	}

	dirp := rip.(*cacheInode)

	zeroinode := 0
	if err := search_dir(dirp, "", &zeroinode, IS_EMPTY); err != nil {
		return false
	}
	return true
}
