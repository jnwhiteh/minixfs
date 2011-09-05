package minixfs

import (
	"testing"
)

// This is a set of functions to test the multi-client support in the minixfs
// implementation.

func TestMultiClient(test *testing.T) {
	fs, proca := OpenMinix3(test)
	procb, err := fs.Spawn(2, 022, "/")
	if err != nil {
		test.Fatalf("Failed creating new process: %s", procb)
	}

	fs.Exit(proca)
	fs.Exit(procb)
	fs.Shutdown()
}
