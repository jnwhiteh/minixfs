package minixfs

const (
	// The number of bits per bitchunk
	FS_BITCHUNK_BITS = uint(Sizeof_bitchunk_t * CHAR_BIT)
)

// The number of bits per block
// #define FS_BITS_PER_BLOCK(b)        (FS_BITMAP_CHUNKS(b) * FS_BITCHUNK_BITS)
func _FS_BITS_PER_BLOCK(b uint) uint {
	return _FS_BITMAP_CHUNKS(b) * FS_BITCHUNK_BITS
}

// The number of bitmap chunks in a block
// #define FS_BITMAP_CHUNKS(b) ((b)/usizeof (bitchunk_t))/* # map chunks/blk   */
func _FS_BITMAP_CHUNKS(b uint) uint {
	return b / Sizeof_bitchunk_t
}
