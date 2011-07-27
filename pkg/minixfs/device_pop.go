package minixfs

import (
	"encoding/binary"
	"os"
)

var ERR_SEEK = os.NewError("could not seek to given position")

type DeviceRequest interface {
	isMessage()
}

type DeviceResponse struct {
	err os.Error
}

type Req_Read struct {
	block interface{}
	pos   int64
}

type Req_Write struct {
	block interface{}
	pos   int64
}

type Req_Scatter struct {
	blocks []*buf
}

type Req_Gather struct {

}

type Req_Close struct {

}

func (m Req_Read) isMessage()    {}
func (m Req_Write) isMessage()   {}
func (m Req_Scatter) isMessage() {}
func (m Req_Gather) isMessage()  {}
func (m Req_Close) isMessage()   {}

// The BlockDevice interface encapsulates the I/O methods of a block device.
// Since we do not have access to raw pointers in typical Go code, the number
// of bytes to be read/written is given by the size of the supplied buffer.
type BlockDevice interface {
	// Open the device
	Start() (chan<- DeviceRequest, <-chan DeviceResponse)
}

// A FileDevice is a block device that is backed by a file on the filesystem.
type FileDevice struct {
	file         *os.File            // the file that represents this device
	filename     string              // the path to the given file
	byteOrder    binary.ByteOrder    // the byte order of the given file
	requestChan  chan DeviceRequest  // a channel on which requests are received
	responseChan chan DeviceResponse // a channel on which responses are sent
}

// NewFileDevice creates a new file-backed block device, given a filename
// and specified byte order.
func NewFileDevice(filename string, byteOrder binary.ByteOrder) (*FileDevice, os.Error) {
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	dev := &FileDevice{file, filename, byteOrder, make(chan DeviceRequest), make(chan DeviceResponse)}

	go dev.loop() // start the main receive loop
	return dev, nil
}

func (dev FileDevice) Start() (chan<- DeviceRequest, <-chan DeviceResponse) {
	requestChan := make(chan DeviceRequest)
	responseChan := make(chan DeviceResponse)

	// Start a goroutine to capture events from the request channel
	go func() {
		for request := range requestChan {
			dev.requestChan <- request
			response, ok := <-dev.responseChan

			// Check to see if the response channel has been closed. If this
			// happens, then the device is closing, so terminate the clients
			// channels as well.
			if !ok {
				close(requestChan)
				close(responseChan)
			} else {
				responseChan <- response
			}
		}
	}()

	return requestChan, responseChan
}

// Main loop that receives requests and returns results
func (dev FileDevice) loop() {
	for request := range dev.requestChan {
		err := dev.process(request)
		dev.responseChan <- DeviceResponse{err}
	}
}

// Process a single request for work
func (dev FileDevice) process(request DeviceRequest) os.Error {
	switch request := request.(type) {
	case Req_Read:
		newPos, err := dev.file.Seek(request.pos, 0)
		if err != nil {
			return err
		} else if request.pos != newPos {
			return ERR_SEEK
		}
		err = binary.Read(dev.file, dev.byteOrder, request.block)
		return err
	case Req_Write:
		newPos, err := dev.file.Seek(request.pos, 0)
		if err != nil {
			return err
		} else if request.pos != newPos {
			return ERR_SEEK
		}

		err = binary.Write(dev.file, dev.byteOrder, request.block)
		return err
	case Req_Scatter:
		panic("NYI: FileDevice.Scatter")
	case Req_Gather:
		panic("NYI: FileDevice.Gather")
	case Req_Close:
		close(dev.requestChan)
		close(dev.responseChan)
	}

	return nil
}
