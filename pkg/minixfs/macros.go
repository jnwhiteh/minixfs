package minixfs

const (
    // #define BITCHUNK_BITS   (sizeof(bitchunk_t) * CHAR_BIT)
	FS_BITCHUNK_BITS = Sizeof_bitchunk_t * CHAR_BIT
)

// #define FS_BITS_PER_BLOCK(b)        (FS_BITMAP_CHUNKS(b) * FS_BITCHUNK_BITS)
func FS_BITS_PER_BLOCK(b int) int {
	return FS_BITMAP_CHUNKS(b) * FS_BITCHUNK_BITS
}

// #define FS_BITMAP_CHUNKS(b) ((b)/usizeof (bitchunk_t))/* # map chunks/blk   */
func FS_BITMAP_CHUNKS(b int) int {
	return b / Sizeof_bitchunk_t
}

