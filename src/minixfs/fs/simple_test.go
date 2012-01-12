package fs

import (
	. "minixfs/testutils"
	"testing"
)

func TestShutdown(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}

	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed shutting down: %s", err)
	}
}
