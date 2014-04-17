package fs

import (
	. "minixfs/testutils"
	"testing"
)

// Change the working directory of the process, and verify that we can open a
// file using the relative path.
func TestChdir(test *testing.T) {
	fs, proc, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}

	// Change into /var and then try to open run/syslogd.pid
	if err = proc.Chdir("/var"); err != nil {
		FatalHere(test, "Failed to change directory: %s", err)
	}
	if proc.workdir.Inum != 543 {
		FatalHere(test, "Got wrong inode expected %d, got %d", 543, proc.workdir.Inum)
	}

	// Fetch something with a relative path
	rip, err := fs.eatPath(proc, "run/syslogd.pid")
	if err != nil {
		FatalHere(test, "Could not open relative file: %s", err)
	}
	if rip.Inum != 481 {
		FatalHere(test, "Got wrong inode expected %d, got %d", 481, rip.Inum)
	}

	fs.itable.PutInode(rip)

	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
