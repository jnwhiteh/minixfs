package minixfs

import (
	"testing"
)

// Try to unlink a file that doesn't exist and ensure that inodes don't leak
func Test_NoFileUnlink(test *testing.T) {
	fs, proc := OpenMinix3(test)
	rip, err := fs.get_inode(ROOT_DEVICE, 538)
	if err != nil {
		test.Errorf("Failed getting /tmp inode: %s", herestr(1))
	}
	if rip.count != 1 {
		test.Errorf("Inode count mismatch, expected %d, got %d: %s", 1, rip.count, herestr(1))
	}

	err = fs.Unlink(proc, "/tmp/SOMERANDOMNAME")
	if err != ENOENT {
		test.Errorf("Expected ENOENT, got %s: %s", err, herestr(1))
	}

	if rip.count != 1 {
		test.Errorf("Inode count mismatch, expected %d, got %d: %s", 1, rip.count, herestr(1))
	}

	fs.put_inode(rip)
	fs.Exit(proc)
	if err := fs.Shutdown(); err != nil {
		test.Errorf("Failed when shutting down filesystem: %s", err)
	}
}

// Try to unlink a directory that doesn't exist and ensure that inodes don't leak
func Test_NoDirUnlink(test *testing.T) {
	fs, proc := OpenMinix3(test)
	rip, err := fs.get_inode(ROOT_DEVICE, 538)
	if err != nil {
		test.Errorf("Failed getting /tmp inode: %s", herestr(1))
	}
	if rip.count != 1 {
		test.Errorf("Inode count mismatch, expected %d, got %d: %s", 1, rip.count, herestr(1))
	}

	err = fs.Rmdir(proc, "/tmp/SOMERANDOMNAME")
	if err != ENOENT {
		test.Errorf("Expected ENOENT, got %s: %s", err, herestr(1))
	}

	if rip.count != 1 {
		test.Errorf("Inode count mismatch, expected %d, got %d: %s", 1, rip.count, herestr(1))
	}

	fs.put_inode(rip)
	fs.Exit(proc)
	if err := fs.Shutdown(); err != nil {
		test.Errorf("Failed when shutting down filesystem: %s", err)
	}
}
