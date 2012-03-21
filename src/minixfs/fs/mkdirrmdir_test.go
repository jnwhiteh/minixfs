package fs

import (
	. "minixfs/common"
	. "minixfs/testutils"
	"testing"
)

// Make a new directory on the file system, ensure that it is given the
// appropriate number/contents, then rmdir the file and check that the file
// system is returned to its initial state.
func TestMkdir(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	bitmap := fs.bitmaps[proc.rootdir.Devnum()]

	// Check the state of the bitmaps before creating this file
	inum, err := bitmap.AllocInode()
	if err != nil {
		FatalHere(test, "Error pre-allocating an inode: %s", err)
	}
	bitmap.FreeInode(inum)

	znum, err := bitmap.AllocZone(NO_ZONE)
	if err != nil {
		FatalHere(test, "Error pre-allocating a zone: %s", err)
	}
	bitmap.FreeZone(znum)

	// Create a new file and check allocation tables, etc.
	err = fs.Mkdir(proc, "/tmp/new_directory", 0666)
	if err != nil {
		FatalHere(test, "Failed when creating new directory: %s", err)
	}
	dirp, err := fs.eatPath(proc, "/tmp/new_directory")
	if err != nil {
		FatalHere(test, "Failed when looking up new directory: %s", err)
	}
	if dirp.Inum() != inum {
		ErrorHere(test, "Inum mismatch expected %d, got %d", inum, dirp.Inum())
	}
	fs.icache.PutInode(dirp)

	// Remove the new directory
	err = fs.Rmdir(proc, "/tmp/new_directory")
	if err != nil {
		ErrorHere(test, "Failed when unlinking new directory: %s", err)
	}

	// The bit we just freed should be our next
	inum2, err := bitmap.AllocInode()
	if err != nil {
		ErrorHere(test, "Failed when checking inode allocation: %s", err)
	}
	if inum != inum2 {
		ErrorHere(test, "Inode mismatch expected %d, got %d", inum, inum2)
	}
	bitmap.FreeInode(inum2)

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
