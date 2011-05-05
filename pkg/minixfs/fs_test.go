package minixfs

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"rand"
	"testing"
	"time"
)

func TestOpen(test *testing.T) {
	fs, err := OpenFileSystemFile("../../minix3root.img")
	if err != nil {
		test.Errorf("Failed to open file system: %s", err)
	}

	if fs.Block_size != 4096 {
		test.Errorf("block size mismatch: got %d, expected %d", fs.Block_size, 4096)
	}
	if fs.Magic != 0x4d5a {
		test.Errorf("magic number mismatch: got 0x%x, expected 0x%x", fs.Magic, 0x4d5a)
	}
}

var _READS_NUM int = 1024
var _READS_SIZE int = 32

func TestRead(test *testing.T) {
	// Read in the original data so we have something to compare against
	file, err := os.Open("../../europarl-en.txt")
	if err != nil {
		test.Logf("Could not open sample file: %s", err)
		test.FailNow()
	}

	odata, err := ioutil.ReadAll(file)
	if err != nil {
		test.Logf("Could not read data from sample file: %s", err)
		test.FailNow()
	}

	// Open the filesystem so we can read from it
	fs, err := OpenFileSystemFile("../../minix3root.img")
	if err != nil || fs == nil {
		test.Errorf("Failed to open file system: %s", err)
	}

	// Register a new process to use as context (umask, rootpath)
	proc, err := fs.NewProcess(1 ,022, "/")
	if err != nil || proc == nil {
		test.Errorf("Failed to register a new process: %s", err)
	}

	// Open the file on the mounted filesystem
	mfile, err := proc.Open("/sample/europarl-en.txt", O_RDONLY, 0666)

	max := len(odata) - _READS_SIZE
	rand.Seed(time.Nanoseconds())

	buf := make([]byte, _READS_SIZE)

	// Run a seek/read test several times
	for i := 0; i < _READS_NUM; i++ {
		// Get a random position to seek to
		pos := rand.Intn(max)

		// Seek to the position in the file, and read a 32-byte block
		npos, err := mfile.Seek(pos, 0)
		if err != nil {
			test.Errorf("Failed when seeking to position %d: %s", pos, err)
		} else if npos != pos {
			test.Errorf("Seek mismatch: got %d, expected %d", npos, pos)
		}

		n, err := mfile.Read(buf)
		if err != nil {
			test.Errorf("Failed when reading from mfile: %s", err)
		} else if n != _READS_SIZE {
			test.Errorf("Len mismatch: got %d, expected %d", n, _READS_SIZE)
		}

		// Check and see if the data matches
		obuf := odata[pos:pos+32]
		if !bytes.Equal(obuf, buf) {
			test.Errorf("Data does not match: got '%q', expected '%q'", buf, obuf)
		}
	}

	log.Printf("Max seek: %d", len(odata) - _READS_SIZE)
}
