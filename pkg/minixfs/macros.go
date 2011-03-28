package minixfs

const (
	// The number of BITS per bitchunk
	// #define BITCHUNK_BITS   (sizeof(bitchunk_t) * CHAR_BIT)
	FS_BITCHUNK_BITS = uint(Sizeof_bitchunk_t * CHAR_BIT)
)

// The number of bits per block
// #define FS_BITS_PER_BLOCK(b)        (FS_BITMAP_CHUNKS(b) * FS_BITCHUNK_BITS)
func FS_BITS_PER_BLOCK(b uint) uint {
	return FS_BITMAP_CHUNKS(b) * FS_BITCHUNK_BITS
}

// The number of bitmap chunks in a block
// #define FS_BITMAP_CHUNKS(b) ((b)/usizeof (bitchunk_t))/* # map chunks/blk   */
func FS_BITMAP_CHUNKS(b uint) uint {
	return b / Sizeof_bitchunk_t
}
