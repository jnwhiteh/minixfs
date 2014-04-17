package fs

import (
	"bytes"
	"io/ioutil"
	. "github.com/jnwhiteh/minixfs/common"
	. "github.com/jnwhiteh/minixfs/testutils"
	"os"
	"testing"
)

// Test write functionality by taking the data from the host system and
// writing it to the guest system along with the host system (at the same
// time) and comparing the returns from these functions. Then compare the
// written data to the original data to make sure it was written correctly.
func TestWrite(test *testing.T) {
	fs, proc := OpenMinixImage(test)
	ofile := OpenEuroparl(test)

	// Read the data for the entire file
	filesize := 4489799 // known
	filedata, err := ioutil.ReadAll(ofile)
	if err != nil {
		FatalHere(test, "Failed when reading from original file: %s", err)
	}
	if filesize != len(filedata) {
		FatalHere(test, "File content sizes differ: %v != %v", len(filedata), filesize)
	}
	if ofile.Close() != nil {
		FatalHere(test, "Failed when closing original file: %s", err)
	}

	// Open the two files that will be written to
	gfile, err := fs.Open(proc, "/tmp/europarl-en.txt", O_CREAT|O_TRUNC|O_RDWR, 0666)
	if err != nil || gfile == nil {
		FatalHere(test, "Could not open file on guest: %s", err)
	}
	hfile, err := os.OpenFile("/tmp/europarl-en.txt", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err != nil || hfile == nil {
		FatalHere(test, "Could not open file on host: %s", err)
	}

	// Write the data to a file on the host/guest operating systems in sync
	blocksize := fs.devinfo[ROOT_DEVICE].Blocksize
	numbytes := blocksize + (blocksize / 3)
	pos := 0

	for pos < filesize {
		// Write the next numbytes bytes to the file
		endpos := pos + numbytes
		if endpos > filesize {
			endpos = filesize
		}
		data := filedata[pos:endpos]

		gn, gerr := gfile.Write(data)
		hn, herr := hfile.Write(data)

		if gn != hn {
			ErrorHere(test, "Bytes read mismatch at offset %d: expected %d, got %d", pos, hn, gn)
		}
		if gerr != herr {
			ErrorHere(test, "Error mismatch at offset %d: expected '%s', got '%s'", pos, herr, gerr)
		}

		rip, err := fs.eatPath(proc, "/tmp/europarl-en.txt")
		if err != nil {
			FatalHere(test, "After write at position %d: could not locate newly created file: %s", pos, err)
		} else {
			fs.itable.PutInode(rip)
		}

		pos += gn
	}

	if hfile.Close() != nil {
		ErrorHere(test, "Failed when closing host file")
	}

	// Seek to beginning of file
	gfile.Seek(0, 0)
	written := make([]byte, filesize)
	n, err := gfile.Read(written)
	if n != filesize {
		ErrorHere(test, "Verify count mismatch expected %d, got %d", filesize, n)
	}
	if err != nil {
		ErrorHere(test, "Error when reading to verify: %s", err)
	}

	compare := bytes.Compare(filedata, written)
	if compare != 0 {
		ErrorHere(test, "Error comparing written data expected %d, got %d", 0, compare)
	}

	if fs.Close(proc, gfile) != nil {
		ErrorHere(test, "Error when closing out the written file: %s", err)
	}

	err = fs.Unlink(proc, "/tmp/europarl-en.txt")
	if err != nil {
		ErrorHere(test, "Failed when unlinking written file: %s", err)
	}

	fs.Exit(proc)
	err = fs.Shutdown()
	if err != nil {
		FatalHere(test, "Failed when shutting down filesystem: %s", err)
	}
}
