package minixfs

import (
	"encoding/binary"
	"os"
	"sync"
	"time"
)

var ERR_SEEK = os.NewError("could not seek to given position")

// The BlockDevice interface encapsulates the I/O methods of a block device.
// Since we do not have access to raw pointers in typical Go code, the number
// of bytes to be read/written is given by the size of the supplied buffer.
type BlockDevice interface {
	// Read will read a block of data from the device from the given position.
	Read(buf interface{}, pos int64) os.Error

	// Write will write a block of data to the device at the given position.
	Write(buf interface{}, pos int64) os.Error

	// Scatter takes a list of cache buffers and will write the given blocks
	// to the disk using some algorithm.
	Scatter([]*buf) os.Error

	Gather() os.Error

	// Close and cleanup the device
	Close()
}

// A FileDevice is a block device that is backed by a file on the filesystem.
type FileDevice struct {
	file      *os.File         // the file that represents this device
	filename  string           // the path to the given file
	byteOrder binary.ByteOrder // the byte order of the given file
	m         *sync.RWMutex
}

// NewFileDevice creates a new file-backed block device, given a filename
// and specified byte order.
func NewFileDevice(filename string, byteOrder binary.ByteOrder) (*FileDevice, os.Error) {
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	return &FileDevice{file, filename, byteOrder, new(sync.RWMutex)}, nil
}

// Read implements the BlockDevice.Read method
func (dev FileDevice) Read(buf interface{}, pos int64) os.Error {
	dev.m.RLock()         // acquire the read mutex
	defer dev.m.RUnlock() // release the read mutex
	newPos, err := dev.file.Seek(pos, 0)
	if err != nil {
		return err
	} else if pos != newPos {
		return ERR_SEEK
	}

	err = binary.Read(dev.file, dev.byteOrder, buf)
	return err
}

// Write implements the BlockDevice.Write method
func (dev FileDevice) Write(buf interface{}, pos int64) os.Error {
	dev.m.Lock() // acquire the write mutex
	defer dev.m.Unlock()
	newPos, err := dev.file.Seek(pos, 0)
	if err != nil {
		return err
	} else if pos != newPos {
		return ERR_SEEK
	}

	err = binary.Write(dev.file, dev.byteOrder, buf)
	return err
}

// Scatter implements the BlockDevice.Scatter method. We take a slice of
// cache blocks that need to be written out to the device and ensure each of
// them are written before returning. This differs significantly from the
// default implementation.
func (dev FileDevice) Scatter(bufq []*buf) os.Error {
	panic("NYI: FileDevice.Scatter")
}

// Gather implements the BlockDevice.Gather method
func (dev FileDevice) Gather() os.Error {
	panic("NYI: FileDevice.Gather")
}

// Close implements the BlockDevice.Close method
func (dev FileDevice) Close() {
	dev.m.Lock() // acquire the write mutex
	defer dev.m.Unlock()
	dev.file.Close()
}

// DelayFileDevice represents a file-backed block device that has a static
// delay for seek operations. There is no explicit locking performed in this
// device, it instead relies on the locking of the underlying FileDevice.
type DelayFileDevice struct {
	seekDelay int64
	dev       *FileDevice
}

// NewDelayFileDevice creates a file-backed block device that has a static
// delay for seek operations.
func NewDelayFileDevice(filename string, byteOrder binary.ByteOrder, seekDelay int64) (*DelayFileDevice, os.Error) {
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	fdev := &FileDevice{file, filename, byteOrder, new(sync.RWMutex)}
	return &DelayFileDevice{seekDelay, fdev}, nil
}

// Read implements the BlockDevice.Read method
func (dev DelayFileDevice) Read(buf interface{}, pos int64) os.Error {
	time.Sleep(dev.seekDelay)
	return dev.dev.Read(buf, pos)
}

// Write implements the BlockDevice.Write method
func (dev DelayFileDevice) Write(buf interface{}, pos int64) os.Error {
	time.Sleep(dev.seekDelay)
	return dev.dev.Write(buf, pos)
}

// Scatter implements the BlockDevice.Scatter method
func (dev DelayFileDevice) Scatter(bufq []*buf) os.Error {
	panic("NYI: DelayFileDevice.Scatter")
}

// Gather implements the BlockDevice.Gather method
func (dev DelayFileDevice) Gather() os.Error {
	panic("NYI: DelayFileDevice.Gather")
}

// Close implements the BlockDevice.Close method
func (dev DelayFileDevice) Close() {
	dev.dev.Close()
}
