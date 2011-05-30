package minixfs

import (
	"encoding/binary"
	"os"
	"testing"
)

// Return the device number corresponding to a given device or NO_DEV
var _getdevnum = func (fs *FileSystem, dev BlockDevice) int {
	fs.m.devs.RLock()
	defer fs.m.devs.RUnlock()

	for i := 0; i < NR_SUPERS; i++ {
		if fs.devs[i] == dev {
			return i
		}
	}
	return NO_DEV
}

func TestMountUnmountUsr(test *testing.T) {
	// Mount the root filesystem
	fs, proc := OpenMinix3(test)
	defer fs.Close()

	// Mount the minix3usr.img on /usr
	dev, err := NewFileDevice("../../minix3usr.img", binary.LittleEndian)
	if err != nil {
		test.Fatalf("Failed to create device for minix3usr.img: %s", err)
	}

	err = fs.Mount(dev, "/usr")
	if err != nil {
		test.Fatalf("Failed to mount minix3usr.img on /dev: %s", err)
	}

	devnum := _getdevnum(fs, dev)
	if devnum == NO_DEV {
		test.Fatalf("Failed looking up device number")
	}

	// Check to see which /usr inode we have (should be the root inode on the
	// new device)
	rip, err := fs.eat_path(proc, "/usr")
	if err != nil {
		test.Fatalf("Could not fetch inode for /usr: %s", err)
	}
	if rip.inum != 1 && rip.dev != devnum {
		test.Fatalf("Mount of /usr not successful, got %q", rip)
	}
	fs.put_inode(rip) // release that inode

	file, err := proc.Open("/usr/pkg/man/man3/SSL_set_fd.3", O_RDONLY, 0)
	if err != nil {
		test.Fatalf("Could not open /usr/pkg/man/man3/SSL_set_fd.3: %s", err)
	}

	if file.inode.inum != 11389 {
		test.Fatalf("Inode mismatch: got %d, expected %d", file.inode.inum, 11389)
	}
	file.Close()

	// Unmount the device
	err = fs.Unmount(dev)
	if err != nil {
		test.Fatalf("Failed when unmounting device: %s", err)
	}

	// Check that the unmount happened properly
	rip, err = fs.eat_path(proc, "/usr")
	if err != nil {
		test.Fatalf("Could not fetch inode for /usr: %s", err)
	}
	if rip.inum != 2 || rip.dev != ROOT_DEVICE {
		test.Fatalf("Unmount of /usr not successful, got %q", rip)
	}
}

func TestMountBad(test *testing.T) {
	// Mount the root filesystem
	fs, _ := OpenMinix3(test)

	var err os.Error

	// Try mounting a busy device (The root one)
	err = fs.Mount(fs.devs[ROOT_DEVICE], "/mnt")
	if err != EBUSY {
		test.Errorf("When mounting busy device, got %s", err)
	}

	// Try mounting a nil device
	err = fs.Mount(nil, "/mnt")
	if err != EINVAL {
		test.Errorf("When mounting nil device, got %s", err)
	}
}

func TestMaxDevices(test *testing.T) {
	// Mount the root filesystem
	fs, proc := OpenMinix3(test)
	defer fs.Close()

	// Mount the same device over and over again on /mnt to fill the table
	path := "/mnt"
	for i := 1; i < NR_SUPERS; i++ {
		dev, err := NewFileDevice("../../minix3root.img", binary.LittleEndian)
		if err != nil {
			test.Fatalf("Failed to create device for minix3usr.img: %s", err)
		}
		err = fs.Mount(dev, path)
		if err != nil {
			test.Fatalf("Failed when mounting copy %d: %s", i, err)
		}
		path += "/mnt"
	}

	// Grab the europarl file from the deepest tree
	file, err := proc.Open("/mnt/mnt/mnt/mnt/mnt/mnt/mnt/sample/europarl-en.txt", O_RDONLY, 0)
	if err != nil {
		test.Fatal("Failed when opening 8 /mnt deep europarl-en.txt: %s", err)
	}
	file.Close()

	// Try to mount another filesystem
	dev, err := NewFileDevice("../../minix3root.img", binary.LittleEndian)
	if err != nil {
		test.Fatalf("Failed to create device for minix3usr.img: %s", err)
	}
	err = fs.Mount(dev, path)
	if err != ENFILE {
		test.Fatalf("When overflowing superblock table, got: %s", err)
	}
}
