package main

const (
	V2_NR_DZONES	= 7		// # direct zone numbers in a V2 inode
	V2_NR_TZONES	= 10	// total # zone numbers in a V2 inode
	MIN_BLOCK_SIZE	= 1024
	MAX_BLOCK_SIZE	= 4096
	SECTOR_SIZE		= 512
	V2_INODE_SIZE	= 64	// manually calculated from d2_inode struct
	INODE_MAX		= 65535
	CHAR_BIT		= 8		// number of bits in a char
	START_BLOCK		= 2		// starting block of FS (not counting SB)
	SUPER_V2		= 0x2468 // magic # for V2 file system
	STATIC_BLOCK_SIZE = 1024
)
