package bitmap

import (
	. "minixfs/common"
)

// Load the contents from disk into map.imap
func (bmap *bitmap) load_map(which int) []bool {
	var start_block int // first bit block
	var map_bits int    // how many bits are there in the bit map
	var bit_blocks int  // how many blocks are there in the bit map

	if which == IMAP {
		start_block = START_BLOCK
		map_bits = bmap.devinfo.Inodes + 1
		bit_blocks = bmap.devinfo.ImapBlocks
	} else {
		start_block = START_BLOCK + bmap.devinfo.ImapBlocks
		map_bits = bmap.devinfo.Zones - (bmap.devinfo.Firstdatazone - 1)
		bit_blocks = bmap.devinfo.ZmapBlocks
	}

	_ = bit_blocks

	result := make([]bool, map_bits)
	bit := 0
	block := 0
	for bit < map_bits {
		bp := bmap.cache.GetBlock(bmap.devno, int(start_block+block), MAP_BLOCK, NORMAL)
		allocTbls := bp.Block.(MapBlock)

		// Iterate over the words in a block
		for word := 0; word < len(allocTbls); word++ {
			num := allocTbls[word]

			// Numbers are little-endian, so the first bit we're looking at is
			// the least-signifigant bit of this number
			for i := uint(0); i < 16 && bit < map_bits; i++ {
				result[bit] = num&(1<<i) != 0
				bit++
			}
		}

		bmap.cache.PutBlock(bp, MAP_BLOCK)
		block++
	}

	return result
}

// Write the contents of a []bool into the bitmap blocks
func (bmap *bitmap) write_map(which int) {
	var start_block int // first bit block
	var map_bits int    // how many bits are there in the bit map
	var bit_blocks int  // how many blocks are there in the bit map
	var list []bool     // the bitmap slice

	if which == IMAP {
		start_block = START_BLOCK
		map_bits = bmap.devinfo.Inodes + 1
		bit_blocks = bmap.devinfo.ImapBlocks
		list = bmap.imap
	} else {
		start_block = START_BLOCK + bmap.devinfo.ImapBlocks
		map_bits = bmap.devinfo.Zones - (bmap.devinfo.Firstdatazone - 1)
		bit_blocks = bmap.devinfo.ZmapBlocks
		list = bmap.zmap
	}

	_ = bit_blocks

	bit := 0
	block := 0
	for bit < map_bits {
		bp := bmap.cache.GetBlock(bmap.devno, int(start_block+block), MAP_BLOCK, NORMAL)
		allocTbls := bp.Block.(MapBlock)

		// Iterate over the words in a block
		for word := 0; word < len(allocTbls); word++ {
			// Need to build up the word value
			num := uint16(0)

			for i := uint(0); i < 16 && bit < map_bits; i++ {
				if list[bit] {
					num = num | (1 << i)
				}
				bit++
			}

			allocTbls[word] = num
		}

		bp.Dirty = true
		bmap.cache.PutBlock(bp, MAP_BLOCK)
		block++
	}
}
