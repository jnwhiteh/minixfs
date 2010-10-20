package minixfs

const (
	CHAR_BIT         = 8  // number of bits in a char
	NR_INODES        = 64 // the number of inodes kept in memory
	ROOT_INODE_NUM   = 1 // the root inode number
	START_BLOCK      = 2  // first block of FS (not counting SB)
	SUPER_V3         = 0x4d5a
	V2_INODE_SIZE    = 64
	V2_NR_DZONES     = 7
	V2_ZONE_NUM_SIZE = 4 // the number of bytes in a zone_t (uint32)
	ZONE_SHIFT       = 0 // unused, but leaving in for clarity
)
