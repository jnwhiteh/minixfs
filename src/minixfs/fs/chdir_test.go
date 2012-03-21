package fs

import (
	. "minixfs/common"
	. "minixfs/testutils"
	"testing"
)

// Change the working directory of the process, and verify that we can open a
// file using the relative path.
func TestChdir(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	// Change into /var and then try to open run/syslogd.pid
	if err = fs.Chdir(proc, "/var"); err != nil {
		FatalHere(test, "Failed to change directory: %s", err)
	}
	if proc.workdir.Inum() != 543 {
		FatalHere(test, "Got wrong inode expected %d, got %d", 543, proc.workdir.Inum())
	}
	_, err = fs.Open(proc, "run/syslogd.pid", O_RDONLY, 0666)
	if err != nil {
		FatalHere(test, "Could not open relative file: %s", err)
	}

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
