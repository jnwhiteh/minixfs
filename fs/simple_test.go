package fs

import (
	"github.com/jnwhiteh/minixfs/common"
	. "github.com/jnwhiteh/minixfs/testutils"
	"testing"
)

func TestSimple(test *testing.T) {
	TestShutdownWithRootProcExit(test)
}

func TestShutdownNoRootProcExit(test *testing.T) {
	fs, _ := OpenMinixImage(test)
	err := fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed shutting down: %s", err)
	}
}

func TestShutdownWithRootProcExit(test *testing.T) {
	fs, proc := OpenMinixImage(test)
	proc.Exit()
	err := fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed shutting down: %s", err)
	}
}

// Ensure cleanup happens properly with fork/exit
func TestForkWithExit(test *testing.T) {
	fs, proc := OpenMinixImage(test)

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
	fs, proc := OpenMinixImage(test)

	_, err := proc.Fork()
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	err = fs.Shutdown()
	if err != common.EBUSY {
		FatalHere(test, "Expected EBUSY error, got: %v", err)
	}
}
