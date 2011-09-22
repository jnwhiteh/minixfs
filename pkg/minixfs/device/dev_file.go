package device

import (
	"encoding/binary"
	. "minixfs/common"
	"os"
)

type fileDevice struct {
	file      *os.File
	filename  string
	byteOrder binary.ByteOrder
	in        chan m_dev_req
	out       chan m_dev_res
}

// NewFileDevice creates a new file-backed block device, given a filename
// and specified byte order.
func NewFileDevice(filename string, byteOrder binary.ByteOrder) (RandDevice, os.Error) {
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	dev := &fileDevice{
		file,
		filename,
		byteOrder,
		make(chan m_dev_req),
		make(chan m_dev_res),
	}

	go dev.loop()
	return dev, nil
}

func (dev *fileDevice) loop() {
	var in <-chan m_dev_req = dev.in
	var out chan<- m_dev_res = dev.out

	for req := range in {
		switch req.call {
		case DEV_READ:
			// device.Read
			newPos, err := dev.file.Seek(req.pos, 0)
			if err != nil {
				out <- m_dev_res{err}
				continue
			} else if req.pos != newPos {
				out <- m_dev_res{ERR_SEEK}
				continue
			}
			err = binary.Read(dev.file, dev.byteOrder, req.buf)
			out <- m_dev_res{err}
		case DEV_WRITE:
			// device.Write
			newPos, err := dev.file.Seek(req.pos, 0)
			if err != nil {
				out <- m_dev_res{err}
				continue
			} else if req.pos != newPos {
				out <- m_dev_res{ERR_SEEK}
				continue
			}
			err = binary.Write(dev.file, dev.byteOrder, req.buf)
			out <- m_dev_res{err}
		case DEV_CLOSE:
			// device.Close
			err := dev.file.Close()
			out <- m_dev_res{err}
			close(dev.in)
			close(dev.out)
		default:
			out <- m_dev_res{ERR_BADCALL}
		}
	}
}

func (dev *fileDevice) Read(buf interface{}, pos int64) os.Error {
	dev.in <- m_dev_req{DEV_READ, buf, pos}
	res := <-dev.out
	return res.err
}

func (dev *fileDevice) Write(buf interface{}, pos int64) os.Error {
	dev.in <- m_dev_req{DEV_WRITE, buf, pos}
	res := <-dev.out
	return res.err
}

func (dev *fileDevice) Close() os.Error {
	dev.in <- m_dev_req{DEV_CLOSE, nil, 0}
	res := <-dev.out
	return res.err
}
