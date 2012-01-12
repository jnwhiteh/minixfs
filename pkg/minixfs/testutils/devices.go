package testutils

import (
	. "../../minixfs/common/_obj/minixfs/common"
	. "../../minixfs/device/_obj/minixfs/device"
	"testing"
)

//////////////////////////////////////////////////////////////////////////////
// A ramdisk device with a certain number of blocks with a given block size.
// Each block is filled with the bytes of the block number, so each byte in
// the first block contains a 0, the next block contains all 1, etc.
//////////////////////////////////////////////////////////////////////////////

func NewTestDevice(test *testing.T, bsize, blocks int) RandDevice {
	data := make([]byte, bsize*blocks)
	for i := 0; i < blocks; i++ {
		for j := 0; j < bsize; j++ {
			data[(i*bsize)+j] = byte(i)
		}
	}
	dev, err := NewRamdiskDevice(data)
	if err != nil {
		ErrorLevel(test, 2, "Failed when creating ramdisk device")
	}
	return dev
}

//////////////////////////////////////////////////////////////////////////////
// A random access device that blocks on any get operation. It notifies of
// the block using the HasBlocked channel and waits to be unblocked on the
// Unblock channel
//////////////////////////////////////////////////////////////////////////////

type BlockingDevice struct {
	RandDevice
	HasBlocked chan bool
	Unblock    chan bool
}

func NewBlockingDevice(rdev RandDevice) *BlockingDevice {
	dev := &BlockingDevice{
		rdev,
		make(chan bool),
		make(chan bool),
	}
	return dev
}

func (dev *BlockingDevice) Read(buf interface{}, pos int64) error {
	dev.HasBlocked <- true
	<-dev.Unblock
	return dev.RandDevice.Read(buf, pos)
}

func (dev *BlockingDevice) Close() error {
	return dev.RandDevice.Close()
}
