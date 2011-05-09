package minixfs

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"testing"
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

type readCase struct {
	name string
	size int
	pos int
}

func TestReadCases(test *testing.T) {
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
		test.Logf("Failed to open file system: %s", err)
		test.FailNow()
	}

	// Register a new process to use as context (umask, rootpath)
	proc, err := fs.NewProcess(1 ,022, "/")
	if err != nil {
		test.Logf("Failed to register a new process: %s", err)
		test.FailNow()
	}

	// Open the file on the mounted filesystem
	mfile, err := proc.Open("/sample/europarl-en.txt", O_RDONLY, 0666)
	log.Printf("Opened file /sample/europarl-en.txt, has size: %v", mfile.rip.Size)
	log.Printf("File is located on inode: %v", mfile.rip.inum)

	// block = position / block_size
	// 0-6 direct blocks (4096 bytes each)
	// 7 indirect block (1024 zone entries, holding 4096 bytes each)
	// 8 doubly indirect block (1024 indb entries, 1024 zones, 4096 bytes each)
	//
	// Maximum file size using direct blocks = 28,672 (28KiB)
	// Maximum file size using indirect+direct = 4,222,976 (4.02 MiB)
	// Maximum file size using dblindirect+etc = 4,299,190,272 (4 GiB)
	//
	// Position 	Block
	// 0-4095 			0 (direct)
	// 4096-8191 		1 (direct)
	// 24576-28671 		6 (direct)

	readCases := []readCase{
		// first block read
		{size: 64, pos: 20000, name: "fbr direct"}, // direct zone (5th block)
		{size: 50, pos: 45000, name: "fbr indirect"}, // indirect zone (11th block)
		{size: 70, pos: 4227000, name: "fbr dblin"}, // doubly indirect (1032) 
	}

	for _, c := range readCases {
		buf := make([]byte, c.size)

		// Seek to the position in the file, and read a 32-byte block
		npos, err := mfile.Seek(c.pos, 0)
		if err != nil {
			test.Errorf("Failed when seeking to position %d: %s", c.pos, err)
		} else if npos != c.pos {
			test.Errorf("Seek mismatch: got %d, expected %d", npos, c.pos)
		}

		// Perform the read
		n, err := mfile.Read(buf)
		if err != nil {
			test.Errorf("Failed when reading from mfile: %s", err)
		} else if n != c.size {
			test.Errorf("Len mismatch: got %d, expected %d", n, c.size)
		}

		// Check and see if the data matches
		obuf := odata[c.pos:c.pos+c.size]
		if !bytes.Equal(obuf, buf) {
			test.Errorf("Data does not match: \n===GOT===\n%s\n===EXPECTED===\n%s\n", buf, obuf)
		}
	}
}
