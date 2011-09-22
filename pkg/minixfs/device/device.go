package device

import (
	"os"
)

var ERR_SEEK = os.NewError("could not seek to given position")
var ERR_BADCALL = os.NewError("bad call")

type CallNumber int

const (
	DEV_READ  CallNumber = iota
	DEV_WRITE CallNumber = iota
	DEV_CLOSE CallNumber = iota
)
