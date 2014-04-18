package fs

import (
	"github.com/jnwhiteh/minixfs/common"
)

func Lookup(rip *common.Inode, name string) (bool, int, int) {
	if !rip.IsDirectory() {
		return false, common.NO_DEV, common.NO_INODE
	}

	dirp := rip

	inum := 0
	err := search_dir(dirp, name, &inum, LOOKUP)
	if err != nil {
		return false, common.NO_DEV, common.NO_INODE
	}

	return true, dirp.Devinfo.Devnum, inum
}

func Link(rip *common.Inode, name string, inum int) error {
	if !rip.IsDirectory() {
		return common.ENOTDIR
	}

	dirp := rip

	// Add the entry to the directory
	err := search_dir(dirp, name, &inum, ENTER)
	return err
}

func Unlink(rip *common.Inode, name string) error {
	if !rip.IsDirectory() {
		return common.ENOTDIR
	}

	dirp := rip

	inum := 0
	err := search_dir(dirp, name, &inum, DELETE)
	return err
}

func IsEmpty(rip *common.Inode) bool {
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
