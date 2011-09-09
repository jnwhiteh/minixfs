package minixfs

import (
	"os"
	"sync"
	"testing"
)

type faultyDevice struct {
	BlockDevice
	blocksize int
	bad       map[int]bool // a map of bad blocks
	release   chan bool
	blocked   chan bool
}

func (dev *faultyDevice) Read(buf interface{}, pos int64) os.Error {
	blockno := int(pos) / dev.blocksize
	if bad, ok := dev.bad[blockno]; ok && bad {
		dev.blocked <- true
		<-dev.release
	}

	return dev.BlockDevice.Read(buf, pos)
}

// Create a device that has a faulty block 2364, which is the first data block
// of the file /sample/europarl-en.txt
func getFaultyMinix3(test *testing.T) (FileSystem, *Process, chan bool, chan bool) {
	// Create a working decide
	dev, err := NewRamdiskDeviceFile("../../minix3root.img")
	if err != nil {
		test.Errorf("Failed to open ramdisk device: %s", err)
	}

	// Wrap it with a faulty device, failing on the first data block of
	// /sample/europarl-en.txt.
	fdev := &faultyDevice{
		dev,
		4096,
		map[int]bool{
			2364: true, // first block of /sample/europarl-en.txt
		},
		make(chan bool),
		make(chan bool),
	}

	fs, err := NewFileSystem(fdev)
	if err != nil {
		test.Errorf("Failed to create new file system: %s", err)
	}
	proc, err := fs.Spawn(1, 022, "/")
	if err != nil {
		test.Logf("Failed to register a new process: %s", err)
		test.FailNow()
	}
	return fs, proc, fdev.release, fdev.blocked
}

// This test checks to see that an open on a device should be able to proceed
// even if a read on another file is blocked waiting for the device. In the
// non-concurrent implementation this will deadlock, but it should pass in a
// correct implementation.
func Test_BlockedRead_Open(test *testing.T) {
	fs, proc, release, blocked := getFaultyMinix3(test)

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		file, err := fs.Open(proc, "/sample/europarl-en.txt", O_RDONLY, 0666)
		if err != nil {
			test.Errorf("Failed when opening file: %s - %s", err, herestr(2))
		}
		buf := make([]byte, 1024)
		file.Read(buf) // this should block
		fs.Close(proc, file)
		wg.Done()
	}()

	go func() {
		// wait for the above to be blocked before proceeding
		<-blocked
		file, err := fs.Open(proc, "/etc/motd", O_RDONLY, 0666)
		if err != nil {
			test.Errorf("Failed when opening file: %s - %s", err, herestr(2))
		}
		fs.Close(proc, file)
		release <- true
		wg.Done()
	}()

	wg.Wait()
	fs.Exit(proc)
	fs.Shutdown()
}

// This test checks to see that a process should be able to chdir even if a
// read on another file is blocked waiting for the device. In the
// non-concurrent implementation this will deadlock, but it should pass in a
// correct implementation.
func Test_BlockedRead_Chdir(test *testing.T) {
	fs, proc, release, blocked := getFaultyMinix3(test)

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		file, err := fs.Open(proc, "/sample/europarl-en.txt", O_RDONLY, 0666)
		if err != nil {
			test.Errorf("Failed when opening file: %s - %s", err, herestr(2))
		}
		buf := make([]byte, 1024)
		file.Read(buf) // this should block
		fs.Close(proc, file)
		wg.Done()
	}()

	go func() {
		<-blocked
		err := fs.Chdir(proc, "/tmp")
		if err != nil {
			test.Errorf("Failed when changing directory: %s - %s", err, herestr(2))
		}
		release <- true
		wg.Done()
	}()

	wg.Wait()
	fs.Exit(proc)
	fs.Shutdown()
}

// This test checks to see that a process should be able to close even if a
// read on another file is blocked waiting for the device. In the
// non-concurrent implementation this will deadlock, but it should pass in a
// correct implementation.
func Test_BlockedRead_Close(test *testing.T) {
	fs, proc, release, blocked := getFaultyMinix3(test)

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		file, err := fs.Open(proc, "/sample/europarl-en.txt", O_RDONLY, 0666)
		if err != nil {
			test.Errorf("Failed when opening file: %s - %s", err, herestr(2))
		}
		buf := make([]byte, 1024)
		file.Read(buf) // this should block
		fs.Close(proc, file)
		wg.Done()
	}()

	go func() {
		file, err := fs.Open(proc, "/etc/motd", O_RDONLY, 0666)
		if err != nil {
			test.Errorf("Failed when opening file: %s - %s", err, herestr(2))
		}
		<-blocked
		fs.Close(proc, file)
		if err != nil {
			test.Errorf("Failed when changing directory: %s - %s", err, herestr(2))
		}
		release <- true
		wg.Done()
	}()

	wg.Wait()
	fs.Exit(proc)
	fs.Shutdown()
}
