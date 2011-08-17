package minixfs

import (
	"sync"
	"testing"
)

const (
	KIBIBYTE = 1024
	MEBIBYTE = 1028 * KIBIBYTE
)

func Test_Ramdisk(test *testing.T) {
	// Create a 20-MiB device for testing
	size := 20 * MEBIBYTE
	data := make([]byte, size, size)
	dev, err := NewRamdiskDevice(data)
	if err != nil {
		test.Errorf("Failed to open ramdisk device: %s", err)
	}

	// Verify that the disk is empty
	for i := 0; i < size; i++ {
		if data[i] != 0 {
			test.Errorf("Failed when verifying byte %d of disk", i)
		}
	}

	magic := make([]byte, 1024, 1024) // 1KiB block
	for i := 0; i < 1024; i++ {
		magic[i] = byte(i)
	}

	// Perform a write of the magic block
	dev.Write(magic, 0)

	// Create a buf
	buf := make([]byte, 1024, 1024) // 1KiB block
	dev.Read(buf, 0)

	// Verify that the block read matches the block written
	for i := 0; i < 1024; i++ {
		if buf[i] != magic[i] {
			test.Errorf("Verifying write on byte %d, got %d, expected %d", i, buf[i], magic[i])
		}
	}

	dev.Close()
}

func Test_RamdiskRead(test *testing.T) {
	dev, err := NewRamdiskDeviceFile("../../minix3root.img")
	if err != nil {
		test.Errorf("Failed to open ramdisk device: %s", err)
	}
	fs, err := NewFileSystem(dev)
	if err != nil {
		test.Errorf("Failed to create new file system: %s", err)
	}

	if fs.supers[0].Block_size != 4096 {
		test.Errorf("block size mismatch: got %d, expected %d", fs.supers[0].Block_size, 4096)
	}
	if fs.supers[0].Magic != 0x4d5a {
		test.Errorf("magic number mismatch: got 0x%x, expected 0x%x", fs.supers[0].Magic, 0x4d5a)
	}

	proc, err := fs.NewProcess(1, 022, "/")
	if err != nil {
		test.Errorf("Failed to create new process: %s", err)
	}

	data := GetEuroparlData(test)
	_Test_Read_Europarl(fs, proc, data, test)
	proc.Exit()
	fs.Close()
}

// Test to ensure that a write cannot happen while a read is outstanding, and
// that reads cannot proceed when a write is outstanding.
func Test_RamdiskWriteblock(test *testing.T) {
	size := 1 * MEBIBYTE
	data := make([]byte, size, size)
	dev := &ramdiskDevice{
		data,
		make(chan BlockRequest),
		make(chan chan BlockResponse),
		new(sync.WaitGroup),
	}
	go dev.loop()

	// We need to lock up the waitgroup with a dummy writer
	dev.rwait.Add(1)

	log := make(chan string, 10)

	// Trigger a write, but it should not happen until we rwait.Done()
	go func() {
		buf := make([]byte, 10, 10)
		err := dev.Write(buf, 0)
		if err != nil {
			test.Errorf("Failed at write %s", err)
		}
		log <- "write"
	}()

	go func() {
		buf := make([]byte, 10)
		err := dev.Read(buf, 0)
		if err != nil {
			test.Errorf("Failed at reada %s", err)
		}
		log <- "reada"
		err = dev.Read(buf, 0)
		if err != nil {
			test.Errorf("Failed at readb %s", err)
		}
		log <- "readb"
	}()

	// TODO: Can we fix the ordering on this?
	dev.rwait.Done()

	if v := <-log; v != "write" {
		test.Errorf("wrong ordering, expected 'write', got %v", v)
	}

	// 'reada' and 'readb' should both happen first
	if v := <-log; v != "reada" {
		test.Errorf("wrong ordering, expected 'reada', got %v", v)
	}

	if v := <-log; v != "readb" {
		test.Errorf("wrong ordering, expected 'readb', got %v", v)
	}

	err := dev.Read(make([]byte, 10), 0)
	if err != nil {
		test.Errorf("Failed at readc: %s", err)
	}

	dev.Close()
}
