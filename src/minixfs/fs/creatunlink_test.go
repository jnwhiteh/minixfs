package fs

import (
	. "minixfs/common"
	. "minixfs/testutils"
	"testing"
)

// Create a new file on the file system, ensure that it is given the
// appropriate inode number and created successfully. Then unlink the file
// so the file system remains in the same state it began in.
func TestCreate(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	bitmap := fs.bitmaps[proc.rootdir.Devnum()]

	// Check the state of the bitmap before creating this file
	inum, err := bitmap.AllocInode()
	if err != nil {
		FatalHere(test, "Error pre-allocating an inode: %s", err)
	}
	bitmap.FreeInode(inum)

	// Get block 13 and print it before we make any changes
	//bp := fs.bcache.GetBlock(ROOT_DEVICE, 13, INODE_BLOCK, NORMAL)
	//debug.PrintBlock(bp, fs.devinfo[ROOT_DEVICE])

	// Create a new file and check allocation tables, etc.
	file, err := fs.Open(proc, "/tmp/created_file", O_CREAT, 0666)
	if err != nil {
		FatalHere(test, "Failed when creating new file: %s", err)
	}
	if file.inode.Inum() != inum {
		ErrorHere(test, "Inum mismatch expected %d, got %d", inum, file.inode.Inum())
	}

	// Close and unlink the new file
	err = fs.Close(proc, file)
	if err != nil {
		ErrorHere(test, "Failed when closing new file: %s", err)
	}
	err = fs.Unlink(proc, "/tmp/created_file")
	if err != nil {
		ErrorHere(test, "Failed when unlinking new file: %s", err)
	}

	// The bit we just freed should be our next
	inum2, err := bitmap.AllocInode()
	if err != nil {
		FatalHere(test, "Failed when checking inode allocation: %s", err)
	}
	if inum != inum2 {
		FatalHere(test, "Inode mismatch expected %d, got %d", inum, inum2)
	}
	bitmap.FreeInode(inum2)

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
