package fs

import (
	"bytes"
	"io"
	"github.com/jnwhiteh/minixfs/common"
	"github.com/jnwhiteh/minixfs/testutils"
	"testing"
)

// Test read functionality by reading the same file from the guest/host
// operating systems and comparing the results of sucessive read calls. The
// number of bytes read per call is set to (4/3) of the block size of the file
// system to ensure that we hit all codepaths.
func TestRead(test *testing.T) {
	fs, proc := OpenMinixImage(test)
	file, err := fs.Open(proc, "/sample/europarl-en.txt", common.O_RDONLY, 0666)
	if err != nil {
		testutils.FatalHere(test, "Failed when opening file: %s", err)
	}

	ofile := OpenEuroparl(test)

	// Read and compare the two files
	blocksize := fs.devinfo[common.ROOT_DEVICE].Blocksize
	numbytes := blocksize + (blocksize / 3)

	data := make([]byte, numbytes)
	odata := make([]byte, numbytes)
	offset := 0

	for {
		n, err := file.Read(data)
		od, oerr := ofile.Read(odata)

		if n != od {
			testutils.FatalHere(test, "Bytes read mismatch at offset %d: expected %d, got %d", offset, od, n)
		}
		if err != oerr {
			testutils.FatalHere(test, "Error mismatch at offset %d: expected '%s', got '%s'", offset, oerr, err)
		}
		if bytes.Compare(data, odata) != 0 {
			testutils.FatalHere(test, "Data mismatch at offset %d\n==Expected\n%s\n==Got\n%s", offset, odata, data)
		}

		if err == io.EOF && oerr == io.EOF {
			break
		}

		offset += n
	}

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		testutils.FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}

// Test changin the position within an open file using the same technique as
// TestRead, by comparing to the POSIX API provided by the Go standard
// libraries corresponding calls.
func TestSeek(test *testing.T) {
	fs, proc := OpenMinixImage(test)

	file, err := fs.Open(proc, "/sample/europarl-en.txt", common.O_RDONLY, 0666)
	if err != nil {
		testutils.FatalHere(test, "Failed when opening file: %s", err)
	}

	ofile := OpenEuroparl(test)

	type seekData struct {
		whence int
		pos    int
	}

	seekOps := []seekData{
		{0, 0},
		{0, 31337},
		{1, 3333},
	}

	// Read and compare several blocks of the file, seeking between each read.
	// This ensures that our seek behaviour is the same as POSIX.
	blocksize := fs.devinfo[common.ROOT_DEVICE].Blocksize
	numbytes := blocksize + (blocksize / 3)

	data := make([]byte, numbytes)
	odata := make([]byte, numbytes)

	for idx, testData := range seekOps {
		pos, err := file.Seek(testData.pos, testData.whence)
		opos, err := ofile.Seek(int64(testData.pos), testData.whence)

		if int64(pos) != opos {
			testutils.FatalHere(test, "Seek position mismatch in test %d: exected %d, got %d", idx, opos, pos)
		}

		n, err := file.Read(data)
		od, oerr := ofile.Read(odata)

		if n != od {
			testutils.FatalHere(test, "Bytes read mismatch at offset %d: expected %d, got %d", idx, od, n)
		}
		if err != oerr {
			testutils.FatalHere(test, "Error mismatch at offset %d: expected '%s', got '%s'", idx, oerr, err)
		}
		if bytes.Compare(data, odata) != 0 {
			testutils.FatalHere(test, "Data mismatch at offset %d\n==Expected\n%s\n==Got\n%s", idx, odata, data)
		}
	}

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		testutils.FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
