package minixfs

import (
	"os"
)

var ERR_SEEK = os.NewError("could not seek to given position")
var ERR_BADCALL = os.NewError("bad call")

type BlockDevice interface {
	Read(buf interface{}, pos int64) os.Error
	Write(buf interface{}, pos int64) os.Error
	Close() os.Error
}

type CallNumber int

const (
	DEV_READ  CallNumber = iota
	DEV_WRITE CallNumber = iota
	DEV_CLOSE CallNumber = iota
)

type BlockRequest struct {
	call CallNumber
	buf  interface{}
	pos  int64
}

type BlockResponse struct {
	err os.Error
}
