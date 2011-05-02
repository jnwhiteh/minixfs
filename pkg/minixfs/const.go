package minixfs

const (
	CHAR_BIT       = 8  // number of bits in a char
	NR_INODES      = 64 // the number of inodes kept in memory
	ROOT_INODE_NUM = 1  // the root inode number
	START_BLOCK    = 2  // first block of FS (not counting SB)

	DEFAULT_NR_BUFS = 1024 // the number of buffer slots in buffer cache

	SUPER_V3 = 0x4d5a

	V2_INODE_SIZE  = 64 // the size of an inode in bytes
	V2_DIRENT_SIZE = 64 // the size of a dirent in bytes

	V2_NR_DZONES     = 7  // number of direct zones in a V2 inode
	V2_NR_TZONES     = 10 // total # of zone numbers in a V2 inode
	V2_ZONE_NUM_SIZE = 4  // the number of bytes in a zone_t (uint32)

	ZONE_SHIFT = 0 // unused, but leaving in for clarity

	IMAP = 0 // operations are on the inode bitmap
	ZMAP = 1 // operations are on the zone bitmap

	NO_ZONE  = 0
	NO_BLOCK = 0
	NO_BIT   = 0

	// When a block is released, the type of usage is passed to put_block()
	WRITE_IMMED = 0100 // block should be written to disk now
	ONE_SHOT    = 0200 // set if block not likely to be needed soon

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
)
