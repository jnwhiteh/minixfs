package minixfs

const (
	CHAR_BIT         = 8 // number of bits in a char
	V2_INODE_SIZE    = 64
	ZONE_SHIFT       = 0 // unused, but leaving in for clarity
	START_BLOCK      = 2 // first block of FS (not counting SB)
	V2_NR_DZONES     = 7
	V2_ZONE_NUM_SIZE = 4 // the number of bytes in a zone_t (uint32)
	SUPER_V3         = 0x4d5a
)
