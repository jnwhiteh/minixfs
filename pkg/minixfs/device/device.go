package device

import "errors"

var ERR_SEEK = errors.New("could not seek to given position")
var ERR_BADCALL = errors.New("bad call")

type CallNumber int

const (
	DEV_READ  CallNumber = iota
	DEV_WRITE CallNumber = iota
	DEV_CLOSE CallNumber = iota
)
