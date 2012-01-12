package bitmap

import (
	. "minixfs/common"
)

func bitmapsize(nr_bits int, block_size int) int {
	// In this assignment, 2 == usizeof(bitchunk_t)
	var bchunks int = block_size / 2
	var bchunk_bits int = 2 * CHAR_BIT
	bitchunks_per_block := bchunks * bchunk_bits

	var nr_blocks int = nr_bits / bitchunks_per_block
	if (nr_blocks * bitchunks_per_block) < nr_bits {
		nr_blocks = nr_blocks + 1
	}
	return nr_blocks
}
