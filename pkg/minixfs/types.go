package minixfs

import "unsafe"

type bit_t uint32
type bitchunk_t uint16
type gid_t byte // char
type ino_t uint32
type mode_t uint16
type nlink_t int16
type off_t int32
type time_t int32
type uid_t int16
type zone_t uint32
type zone1_t uint16

const (
	Sizeof_bitchunk_t = uint(unsafe.Sizeof(*new(bitchunk_t)))
)
