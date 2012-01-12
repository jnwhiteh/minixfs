package fs

import (
	. "minixfs/common"
	. "minixfs/testutils"
	"testing"
)

// Ensure cleanup happens properly with spawn/exit
func TestSpawnExit(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}
	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}

// Ensure that an open process prevents clean shutdown
func TestSpawnNoExit(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	_ = proc

	err = fs.Shutdown()
	if err != EBUSY {
		FatalHere(test, "Expected EBUSY error, got: %s", err)
	}
}
