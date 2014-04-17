package fs

import (
	"encoding/binary"
	"github.com/jnwhiteh/minixfs/device"
	. "github.com/jnwhiteh/minixfs/testutils"
	"testing"
)

// Test that mounting a device (on /mnt) functions properly
func TestMount(test *testing.T) {
	fs, proc := OpenMinixImage(test)

	// Create a secondary device to mount
	imageFilename := getExtraFilename("minix3root.img")
	dev, err := device.NewFileDevice(imageFilename, binary.LittleEndian)
	if err != nil {
		FatalHere(test, "Failed when creating new device: %s", err)
	}

	// Mount it on /mnt, so that is a mirror of the root filesystem
	err = fs.Mount(proc, dev, "/mnt")
	if err != nil {
		FatalHere(test, "Failed when mounting: %s", err)
	}

	rip, err := fs.eatPath(proc, "/mnt/sample/europarl-en.txt")
	if err != nil {
		FatalHere(test, "Failed fetching inode: %s", err)
	}

	if rip.Inum != 542 {
		FatalHere(test, "Data mismatch for inum got %d, expected %d", rip.Inum, 542)
	}
	if rip.Nlinks != 1 {
		FatalHere(test, "Data mismatch for links got %d, expected %d", rip.Nlinks, 1)
	}
	if rip.Size != 4489799 {
		FatalHere(test, "Data mismatch for size got %d, expected %d", rip.Size, 4489799)
	}

	fs.itable.PutInode(rip)

	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}

// Test that unmounting restores the file system to normal
func TestUnmount(test *testing.T) {
	fs, proc := OpenMinixImage(test)

	// Create a secondary device to mount
	imageFilename := getExtraFilename("minix3root.img")
	dev, err := device.NewFileDevice(imageFilename, binary.LittleEndian)
	if err != nil {
		FatalHere(test, "Failed when creating new device: %s", err)
	}

	// Mount it on /mnt, so that is a mirror of the root filesystem
	err = fs.Mount(proc, dev, "/mnt")
	if err != nil {
		FatalHere(test, "Failed when mounting: %s", err)
	}
	err = fs.Unmount(proc, "/mnt")
	if err != nil {
		FatalHere(test, "Failed when unmounting: %s", err)
	}

	// Now check to make sure that things have been returned to normal
	rip, err := fs.eatPath(proc, "/mnt")
	if err != nil {
		FatalHere(test, "Could not fetch path /mnt: %s", err)
	}

	if rip.Inum != 518 {
		FatalHere(test, "Data mismatch for inum got %d, expected %d", rip.Inum, 518)
	}
	if rip.Nlinks != 2 {
		FatalHere(test, "Data mismatch for links got %d, expected %d", rip.Nlinks, 2)
	}
	if rip.Size != 128 {
		FatalHere(test, "Data mismatch for size got %d, expected %d", rip.Size, 128)
	}

	fs.itable.PutInode(rip)

	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
