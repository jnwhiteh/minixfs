package fs

import (
	"minixfs2/common"
	. "minixfs2/testutils"
	"testing"
)

func TestSimple(test *testing.T) {
	TestShutdownWithRootProcExit(test)
}

func TestShutdownNoRootProcExit(test *testing.T) {
	fs, _, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}

	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed shutting down: %s", err)
	}
}

func TestShutdownWithRootProcExit(test *testing.T) {
	fs, proc, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}

	proc.Exit()
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed shutting down: %s", err)
	}
}

// Ensure cleanup happens properly with fork/exit
func TestForkWithExit(test *testing.T) {
	fs, proc, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}

	child, err := proc.Fork()
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	child.Exit()
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}

// Ensure that an open process prevents clean shutdown
func TestSpawnNoExit(test *testing.T) {
	fs, proc, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}

	_, err = proc.Fork()
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	err = fs.Shutdown()
	if err != common.EBUSY {
		FatalHere(test, "Expected EBUSY error, got: %v", err)
	}
}
