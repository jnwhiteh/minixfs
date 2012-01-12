package fs

import (
	. "minixfs/common"
	. "minixfs/testutils"
	"testing"
)

// Test that we can open a file an it is added to the files/filp tables
func TestOpen(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	file, err := fs.Open(proc, "/sample/europarl-en.txt", O_RDONLY, 0666)
	if err != nil {
		FatalHere(test, "Failed opening file: %s", err)
	}

	// Verify open file count and presence of *File entry
	found := false
	count := 0
	for _, fi := range proc.files {
		if fi == file {
			found = true
		}
		if fi != nil {
			count++
		}
	}

	if !found {
		FatalHere(test, "Did not find open file in proc.files")
	}
	if count != 1 {
		FatalHere(test, "Open file count incorrect got %d, expected %d", count, 1)
	}

	if file.count != 1 {
		FatalHere(test, "Filp count wrong got %d, expected %d", file.count, 1)
	}

	// Check to make sure there is a global filp entry
	found = false
	count = 0
	for _, fi := range fs.filps {
		if fi == file.Filp {
			found = true
		}
		if fi != nil {
			count++
		}
	}

	if !found {
		FatalHere(test, "Did not find global filp entry")
	}
	if count != 1 {
		FatalHere(test, "Global filp count wrong got %d, expected %d", count, 1)
	}

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}

// Test that we close a file it is removed from fd/filp tables
func TestClose(test *testing.T) {
	fs, err := OpenFileSystemFile("../../../minix3root.img")
	if err != nil {
		FatalHere(test, "Failed opening file system: %s", err)
	}
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		FatalHere(test, "Failed when spawning new process: %s", err)
	}

	file, err := fs.Open(proc, "/sample/europarl-en.txt", O_RDONLY, 0666)
	if err != nil {
		FatalHere(test, "Failed opening file: %s", err)
	}

	err = fs.Close(proc, file)
	if err != nil {
		FatalHere(test, "Failed closing file: %s", err)
	}

	// Verify open file count and presence of *File entry
	found := false
	count := 0
	for _, fi := range proc.files {
		if fi == file {
			found = true
		}
		if fi != nil {
			count++
		}
	}

	if found {
		FatalHere(test, "Found closed file in proc.files")
	}
	if count != 0 {
		FatalHere(test, "Open file count incorrect got %d, expected %d", count, 0)
	}

	// Check to make sure the filp is not in the global table anymore
	found = false
	count = 0
	for _, fi := range fs.filps {
		if fi == file.Filp && fi != nil {
			found = true
		}
		if fi != nil {
			count++
		}
	}

	if found {
		FatalHere(test, "Found global filp entry for closed file")
	}
	if count != 0 {
		FatalHere(test, "Global filp count wrong got %d, expected %d", count, 0)
	}

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
