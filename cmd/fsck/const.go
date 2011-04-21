package main

const (
	SUPER_MAGIC  = 0x137F // magic number contained in super-block
	SUPER_REV    = 0x7F13 // magic # when 68000 disk read on PC or vv
	SUPER_V2     = 0x2468 // magic # for V2 file systems
	SUPER_V2_REV = 0x6824 // V2 magic written on PC, read on 68K or vv
	SUPER_V3     = 0x4d5a // magic # for V3 file systems

	ROOT_INODE = 1

	V2_INODE_SIZE    = 64
	V2_NR_DZONES     = 7
	V2_ZONE_NUM_SIZE = Sizeof_zone_t

	MAX_FILE_POS = 0x7FFFFFFF // largest legal file offset

	_MIN_BLOCK_SIZE = 1024 // the minimum block size

	BITSHIFT   = 5
	NAME_MAX   = DIRSIZ
	DIRSIZ     = 60
	INODE_SIZE = V2_INODE_SIZE

	I_TYPE          = 0170000 // bit mask for type of inode
	I_UNIX_SOCKET   = 0140000 // unix domain socket
	I_SYMBOLIC_LINK = 0120000 // file is a symbolic link
	I_REGULAR       = 0100000 // regular file, not dir or special
	I_BLOCK_SPECIAL = 0060000 // block special file
	I_DIRECTORY     = 0040000 // file is a directory
	I_CHAR_SPECIAL  = 0020000 // character special file
	I_NAMED_PIPE    = 0010000 // named pipe (FIFO)
	I_SET_UID_BIT   = 0004000 // set effective uid_t on exec
	I_SET_GID_BIT   = 0002000 // set effective gid_t on exec
	I_SET_STCKY_BIT = 0001000 // sticky bit
	ALL_MODES       = 0007777 // all bits for user, group and others
	RWX_MODES       = 0000777 // mode bits for RWX only
	R_BIT           = 0000004 // Rwx protection bit
	W_BIT           = 0000002 // rWx protection bit
	X_BIT           = 0000001 // rwX protection bit
	I_NOT_ALLOC     = 0000000 // this inode is free
	STICKY_BIT      = 01000

	MAJOR = 8
	MINOR = 0

	DOT    = 1
	DOTDOT = 2

	NO_ZONE = 0
)
