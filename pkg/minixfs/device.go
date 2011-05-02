package minixfs

import (
	"encoding/binary"
	"os"
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
}

// A FileDevice is a block device that is backed by a file on the filesystem.
type FileDevice struct {
	file      *os.File         // the file that represents this device
	filename  string           // the path to the given file
	byteOrder binary.ByteOrder // the byte order of the given file
}

// NewFileDevice creates a new file-backed block device, given a filename
// and specified byte order.
func NewFileDevice(filename string, byteOrder binary.ByteOrder) (*FileDevice, os.Error) {
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	return &FileDevice{file, filename, byteOrder}, nil
}

// Read implements the BlockDevice.Read method
func (dev FileDevice) Read(buf interface{}, pos int64) os.Error {
	newPos, err := dev.file.Seek(pos, 0)
	if err != nil {
		return err
	} else if pos != newPos {
		return ERR_SEEK
	}

	err = binary.Read(dev.file, dev.byteOrder, buf)
	return err
}

func (dev FileDevice) Write(buf interface{}, pos int64) os.Error {
	newPos, err := dev.file.Seek(pos, 0)
	if err != nil {
		return err
	} else if pos != newPos {
		return ERR_SEEK
	}

	err = binary.Write(dev.file, dev.byteOrder, buf)
	return err
}

func (dev FileDevice) Scatter(bufq []*buf) os.Error {
	panic("NYI: FileDevice.Scatter")
}

func (dev FileDevice) Gather() os.Error {
	panic("NYI: FileDevice.Gather")
}
