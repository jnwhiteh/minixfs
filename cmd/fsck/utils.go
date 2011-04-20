package main

import "unsafe"

type bitchunk_t uint32
type block_nr block_t
type block_t uint32
type zone_t uint32

const (
	Sizeof_zone_t = unsafe.Sizeof(*new(zone_t))
	Sizeof_block_nr = unsafe.Sizeof(*new(block_nr))
	Sizeof_bitchunk_t = unsafe.Sizeof(*new(bitchunk_t))
	CHAR_BIT       = 8  // number of bits in a char
	FS_BITCHUNK_BITS = Sizeof_bitchunk_t * CHAR_BIT
	BITMASK = (1 << BITSHIFT) - 1
)
func FS_BITS_PER_BLOCK(b int) int {
	return FS_BITMAP_CHUNKS(b) * FS_BITCHUNK_BITS
}

func FS_BITMAP_CHUNKS(b int) int {
	return b / Sizeof_bitchunk_t
}

func V2_INDIRECTS() int {
	return block_size / V2_ZONE_NUM_SIZE
}

func MAX_ZONES(b int) int {
	return V2_NR_DZONES + V2_INDIRECTS() + V2_INDIRECTS() * V2_INDIRECTS()
}

func bitmapsize(nr_bits int, block_size int) int {
	nr_blocks := nr_bits / FS_BITS_PER_BLOCK(block_size)
	if (nr_blocks * FS_BITS_PER_BLOCK(block_size)) < nr_bits {
		nr_blocks++
	}
	return nr_blocks
}

func WORDOFBIT(b int) int {
	return b >> BITSHIFT
}

func POWEROFBIT(b int) int {
	return (1 << uint(b & BITMASK))
}

func setbit(w []bitchunk_t, b int) {
	w[WORDOFBIT(b)] |= bitchunk_t(POWEROFBIT(b))
}

func clrbit(w []bitchunk_t, b int) {
	w[WORDOFBIT(b)] &= bitchunk_t(^POWEROFBIT(b))
}

// Test if a bit is set
func bitset(w []bitchunk_t, b int) bool {
	return (w[WORDOFBIT(b)] & bitchunk_t(POWEROFBIT(b))) != 0
}

func BLK_ILIST() int {
	BLK_IMAP := 2
	N_IMAP := int(sb.Imap_blocks)
	N_ZMAP := int(sb.Zmap_blocks)
	BLK_ZMAP := BLK_IMAP + N_IMAP
	return BLK_ZMAP + N_ZMAP
}

func inoblock(inn int) int {
	return (((inn - 1) * INODE_SIZE) / block_size) + BLK_ILIST()
}

func inooff(inn int) int {
	return ((inn - 1) * INODE_SIZE) / block_size
}

func ZONE_SIZE() int {
	return ztob(block_size)
}

func ztob(z int) int {
	return z << sb.Log_zone_size
}

func FIRST() int {
	return sb.Firstdatazone
}
