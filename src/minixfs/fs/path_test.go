package fs

import (
	. "minixfs/testutils"
	"testing"
)

// Test that path lookups function properly
func TestEatPath(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	// Fetch some additional inodes to ensure path lookup is functioning
	// properly.
	//
	//  inode permission link   size name
	//      1 drwxr-xr-x  14    1088 /
	//      2 drwxr-xr-x   2     128 /usr
	//    541 drwxr-xr-x   2     192 /sample
	//    542 -rw-r--r--   1 4489799 /sample/europarl-en.txt
	//     35 -rw-------   2 2705920 /boot/image/3.1.8
	//    540 -rw-r--r--   1     395 /root/.ssh/known_hosts
	//    481 -rw-------   1       5 /var/run/syslogd.pid

	type inodeTest struct {
		path  string
		inum  int
		links int
		size  int
		zones []int
	}

	inodeTests := []inodeTest{
		{"/", 1, 14, 1088, nil},
		{"/usr", 2, 2, 128, nil},
		{"/sample", 541, 2, 192, nil},
		{"/sample/europarl-en.txt", 542, 1, 4489799, nil},
		{"/boot/image/3.1.8", 35, 2, 2705920, nil},
		{"/root/.ssh/known_hosts", 540, 1, 395, nil},
		{"/var/run/syslogd.pid", 481, 1, 5, nil},
	}

	for _, itest := range inodeTests {
		rip, err := fs.eatPath(proc, itest.path)
		if err != nil {
			FatalHere(test, "Failed when fetching inode for %s: %s", itest.path, err)
		}
		if itest.inum != -1 && rip.Inum != itest.inum {
			ErrorHere(test, "[%s] mismatch for inum got %d, expected %d", itest.path, rip.Inum, itest.inum)
		}
		if itest.links != -1 && rip.Inode.Nlinks != uint16(itest.links) {
			ErrorHere(test, "[%s] mismatch for links got %d, expected %d", itest.path, rip.Inode.Nlinks, itest.links)
		}
		if itest.size != -1 && rip.Inode.Size != int32(itest.size) {
			ErrorHere(test, "[%s] mismatch for size got %d, expected %d", itest.path, rip.Inode.Size, itest.size)
		}
		for i := 0; i < 10; i++ {
			if i < len(itest.zones) && rip.Inode.Zone[i] != uint32(itest.zones[i]) {
				ErrorHere(test, "[%s] mismatch for zone[%d] got %d, expected %d", i, itest.path, rip.Inode.Zone[i], itest.zones[i])
			}
		}
		fs.icache.PutInode(rip)
	}

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
