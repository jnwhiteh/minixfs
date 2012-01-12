package fs

import (
	"encoding/binary"
	device "minixfs/device"
	. "minixfs/testutils"
	"testing"
)

// Test that mounting a device (on /mnt) functions properly
func TestMount(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}

	// Create a secondary device to mount
	dev, err := device.NewFileDevice("../../../minix3root.img", binary.LittleEndian)
	if err != nil {
		FatalHere(test, "Failed when creating new device: %s", err)
	}

	// Mount it on /mnt, so that is a mirror of the root filesystem
	err = fs.Mount(dev, "/mnt")
	if err != nil {
		FatalHere(test, "Failed when mounting: %s", err)
	}

	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		FatalHere(test, "Failed spawning new process: %s", err)
	}
	rip, err := fs.eatPath(proc, "/mnt/sample/europarl-en.txt")
	if err != nil {
		FatalHere(test, "Failed fetching inode: %s", err)
	}

	if rip.Inum != 542 {
		FatalHere(test, "Data mismatch for inum got %d, expected %d", rip.Inum, 542)
	}
	if rip.Inode.Nlinks != 1 {
		FatalHere(test, "Data mismatch for links got %d, expected %d", rip.Inode.Nlinks, 1)
	}
	if rip.Inode.Size != 4489799 {
		FatalHere(test, "Data mismatch for size got %d, expected %d", rip.Inode.Size, 4489799)
	}

	fs.icache.PutInode(rip)
	fs.Exit(proc)

	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}

// Test that unmounting restores the file system to normal
func TestUnmount(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}

	// Create a secondary device to mount
	dev, err := device.NewFileDevice("../../../minix3root.img", binary.LittleEndian)
	if err != nil {
		FatalHere(test, "Failed when creating new device: %s", err)
	}

	// Mount it on /mnt, so that is a mirror of the root filesystem
	err = fs.Mount(dev, "/mnt")
	if err != nil {
		FatalHere(test, "Failed when mounting: %s", err)
	}
	err = fs.Unmount(dev)
	if err != nil {
		FatalHere(test, "Failed when unmounting: %s", err)
	}

	proc, err := fs.Spawn(1, 022, "/mnt")
	if err != nil {
		FatalHere(test, "Failed spawning new process: %s", err)
	}
	rip := proc.rootdir
	if rip.Inum != 518 {
		FatalHere(test, "Data mismatch for inum got %d, expected %d", rip.Inum, 518)
	}
	if rip.Inode.Nlinks != 2 {
		FatalHere(test, "Data mismatch for links got %d, expected %d", rip.Inode.Nlinks, 2)
	}
	if rip.Inode.Size != 128 {
		FatalHere(test, "Data mismatch for size got %d, expected %d", rip.Inode.Size, 128)
	}

	fs.Exit(proc)

	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
