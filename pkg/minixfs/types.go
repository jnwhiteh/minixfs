package minixfs

import "unsafe"

type bitchunk_t uint16

const (
	Sizeof_bitchunk_t = unsafe.Sizeof(*new(bitchunk_t))
)
