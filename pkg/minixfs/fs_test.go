package minixfs

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

// Open the sample minix3 file system and create a process '1'
func OpenMinix3(test *testing.T) (*fileSystem, *Process) {
	// Open the filesystem so we can read from it
	fs, err := OpenFileSystemFile("../../minix3root.img")
	if err != nil || fs == nil {
		test.Logf("Failed to open file system: %s", err)
		test.FailNow()
	}

	// Register a new process to use as context (umask, rootpath)
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		test.Logf("Failed to register a new process: %s", err)
		test.FailNow()
	}

	return fs, proc
}

func TestOpen(test *testing.T) {
	fs, err := OpenFileSystemFile("../../minix3root.img")
	if err != nil {
		test.Logf("Failed to open file system: %s", err)
		test.FailNow()
	}

	if fs.supers[0].Block_size != 4096 {
		test.Errorf("block size mismatch: got %d, expected %d", fs.supers[0].Block_size, 4096)
	}
	if fs.supers[0].Magic != 0x4d5a {
		test.Errorf("magic number mismatch: got 0x%x, expected 0x%x", fs.supers[0].Magic, 0x4d5a)
	}
	if err := fs.Shutdown(); err != nil {
		test.Error("Failed when shutting down filesystem: %s", err)
	}
}

type readCase struct {
	name string
	size int
	pos  int
}

func GetEuroparlData(test *testing.T) []byte {
	// Read in the original data so we have something to compare against
	dfile, err := os.Open("../../europarl-en.txt")

	if err != nil {
		test.Logf("Could not open sample file: %s", err)
		test.FailNow()
	}

	odata, err := ioutil.ReadAll(dfile)
	if err != nil {
		test.Logf("Could not read data from sample file: %s", err)
		test.FailNow()
	}
	dfile.Close()

	return odata
}

func openEuroparl(test *testing.T) (*fileSystem, *Process, []byte, *File) {
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
	file.Close()

	fs, proc := OpenMinix3(test)

	// Open the file on the mounted filesystem
	mfile, err := fs.Open(proc, "/sample/europarl-en.txt", O_RDONLY, 0666)
	//log.Printf("Opened file /sample/europarl-en.txt, has size: %v", mfile.inode.Size)
	//log.Printf("File is located on inode: %v", mfile.inode.inum)

	return fs, proc, odata, mfile
}

func TestReadCases(test *testing.T) {
	fs, proc, odata, mfile := openEuroparl(test)
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
		{size: 64, pos: 20000, name: "fbr direct"},   // direct zone (5th block)
		{size: 50, pos: 45000, name: "fbr indirect"}, // indirect zone (11th block)
		{size: 70, pos: 4227000, name: "fbr dblin"},  // doubly indirect (1032)
		// first block read (full block)
		{size: 4096, pos: 4096, name: "fbrfb direct"},    // direct zone (full second block)
		{size: 4096, pos: 40960, name: "fbrfb indirect"}, // indirect zone (full 10th block)
		{size: 4096, pos: 4227072, name: "fbrfb dblin"},  // double indirect (full 1032nd block)
		// partial last block
		{size: 6144, pos: 4096, name: "plb direct"},    // direct zone (1.5 blocks)
		{size: 6144, pos: 40960, name: "plb indirect"}, // indirect zone (1.5 blocks)
		{size: 6144, pos: 4227072, name: "plb dblin"},  // double indirect (1.5 blocks)
		// the following tests failed when they were 'random'
		{size: 4691, pos: 2035452, name: "random 1"},
	}

	total := 0
	for _, c := range readCases {
		//log.Printf("Running test case %d - %s", idx, c.name)
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
		obuf := odata[c.pos : c.pos+c.size]
		if !bytes.Equal(obuf, buf) {
			test.Errorf("Data integrity for test '%s' failed", c.name)
		}
		total += c.size
	}

	fs.Exit(proc)
	if err := fs.Shutdown(); err != nil {
		test.Error("Failed when shutting down filesystem: %s", err)
	}

	//log.Printf("Checked a total of %d bytes in %d read cases", total, len(readCases))
}
