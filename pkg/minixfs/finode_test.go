package minixfs

import (
	"testing"
)

func Test_Finode_Shutdown(test *testing.T) {
	// Ensure that Finode processes are properly spawned and shut down

	fs, proc := OpenMinix3(test)
	file, err := fs.Open(proc, "/sample/europarl-en.txt", O_RDONLY, 066)
	if err != nil {
		test.Errorf("Failed when opening file: %s", err)
	}

	finode, ok := fs.finodes[file.inode]
	if !ok {
		test.Errorf("New finode was not spawned")
	}
	if len(fs.finodes) != 1 {
		test.Errorf("Wrong finode count, expected 1, got %d", len(fs.finodes))
	}
	if finode.count != 1 {
		test.Errorf("Wrong finode open count, expected 1, got %d", finode.count)
	}

	// Open a second copy
	file2, err := fs.Open(proc, "/sample/europarl-en.txt", O_RDONLY, 066)
	if len(fs.finodes) != 1 {
		test.Errorf("Wrong finode count, expected 1, got %d", len(fs.finodes))
	}
	if finode.count != 2 {
		test.Errorf("Wrong finode open count, expected 2, got %d", finode.count)
	}

	fs.Close(proc, file2)
	if len(fs.finodes) != 1 {
		test.Errorf("Wrong finode count, expected 1, got %d", len(fs.finodes))
	}
	if finode.count != 1 {
		test.Errorf("Wrong finode open count, expected 1, got %d", finode.count)
	}

	rip := file.inode
	fs.Close(proc, file)
	if len(fs.finodes) != 0 {
		test.Errorf("Wrong finode count, expected 0, got %d", len(fs.finodes))
	}
	if finode.count != 0 {
		test.Errorf("Wrong finode open count, expected 0, got %d", finode.count)
	}
	finode, ok = fs.finodes[rip]
	if ok {
		test.Errorf("Expected !ok, got %v: %p", ok, finode)
	}

	fs.Exit(proc)
	fs.Shutdown()
}
