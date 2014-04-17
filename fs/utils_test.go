package fs

import (
	"github.com/jnwhiteh/minixfs/testutils"
	"os"
	"path"
	"testing"
)

func getExtraFilename(filename string) string {
	dir := os.Getenv("EXTRA_PATH")
	if len(dir) == 0 {
		dir = "/tmp"
	}
	return path.Join(dir, filename)
}

func OpenMinixImage(test *testing.T) (*FileSystem, *Process) {
	imageFilename := getExtraFilename("minix3root.img")
	fs, proc, err := OpenFileSystemFile(imageFilename)
	if err != nil {
		testutils.FatalHere(test, "Failed opening file system: %s", err)
	}
	return fs, proc
}

func OpenEuroparl(test *testing.T) *os.File {
	filename := getExtraFilename("europarl-en.txt")
	file, err := os.OpenFile(filename, os.O_RDONLY, 0666)
	if err != nil {
		testutils.FatalHere(test, "Could not open Europarl reference: %s", err)
	}
	return file
}
