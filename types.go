package main

// overall types
type short uint16
type ushort int16
type long int32
type ulong uint32

// types defined in main.go
type mode_t ushort
type uid_t short
type off_t long
type time_t ulong
type zone_t uint32
type block_t uint32
type bitchunk_t uint16

// types defined in inode.go
type nlink_t short
type gid_t byte

// types defined in super.go

type bit_t uint32
type ino_t ulong
type zone1_t uint16
type dev_t short



