package fs

import (
	. "github.com/jnwhiteh/minixfs/common"
	. "github.com/jnwhiteh/minixfs/testutils"
	"runtime"
	"testing"
)

func checkFileAndCount(proc *Process, file Fd) (bool, int) {
	filp := file.(*filp)

	// Verify open file count and presence of *File entry
	found := false
	count := 0
	for _, fi := range proc.files {
		if fi == filp {
			found = true
		}
		if fi != nil {
			count++
		}
	}

	return found, count
}

// Test that we can open and close a file
func TestOpenClose(test *testing.T) {
	fs, proc := OpenMinixImage(test)

	file, err := proc.Open("/sample/europarl-en.txt", O_RDONLY, 0666)
	if err != nil {
		FatalHere(test, "Failed opening file: %s", err)
	}

	found, count := checkFileAndCount(proc, file)

	if !found {
		FatalHere(test, "Did not find open file in proc.files")
	}
	if count != 1 {
		FatalHere(test, "Open file count incorrect got %d, expected %d", count, 1)
	}

	// Now close the file and make sure things are cleaned up
	err = proc.Close(file)

	found, count = checkFileAndCount(proc, file)

	if found {
		FatalHere(test, "Found file in process table, should not have")
	}
	if count != 0 {
		FatalHere(test, "Open file count mismatch got %d, expected %d", count, 0)
	}

	// How many goroutines are open right now?
	numgoros := runtime.NumGoroutine()
	stacknow := make([]byte, 4096)
	runtime.Stack(stacknow, true)

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}

	// We expect shutdown to have killed the following goroutines
	//  * device
	//  * block cache
	//  * inode cache
	//  * allocation table
	//  * file server

	// This test is fragile, so be careful with it!
	expected := numgoros - 5
	if runtime.NumGoroutine() != expected {
		test.Logf("Original stack:\n%s\n", stacknow)
		newstack := make([]byte, 4096)
		runtime.Stack(newstack, true)
		test.Logf("Current stack:\n%s\n", newstack)
		FatalHere(test, "Goroutine count mismatch got %d, expected %d", expected, runtime.NumGoroutine())
	}
}
