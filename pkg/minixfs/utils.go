package minixfs

import "log"
import "math"

// This utility package assumes that each file system is a read/write file
// system and does not contain any mounted sub-filesystems. If either of
// these conditions is violated, the code will need to be adjusted.

// Allocate a bit from a bit map and return its bit number
func (fs *FileSystem) AllocBit(bmap uint, origin uint) (uint) {
	var start_block uint // first bit block
	var map_bits uint    // how many bits are there in the bit map
	var bit_blocks uint // how many blocks are there in the bit map

	if bmap == IMAP {
		start_block = START_BLOCK
		map_bits = fs.super.Ninodes + 1
		bit_blocks = fs.super.Imap_blocks
	} else {
		start_block = START_BLOCK + fs.super.Imap_blocks
		map_bits = fs.super.Zones - (fs.super.Firstdatazone_old-1)
		bit_blocks = fs.super.Zmap_blocks
	}

	// Figure out where to start the bit search (depends on 'origin')
	if origin >= map_bits {
		origin = 0 // for robustness
	}

	// Locate the starting place
	block := origin / FS_BITS_PER_BLOCK(fs.Block_size)
	word := (origin % FS_BITS_PER_BLOCK(fs.Block_size)) / FS_BITCHUNK_BITS

	// Iterate over all blocks plus one, because we start in the middle
	bcount := bit_blocks + 1
	bmapb := make([]uint16, FS_BITMAP_CHUNKS(fs.Block_size))
	//wlim := FS_BITMAP_CHUNKS(fs.Block_size)

	for {
		err := fs.GetBlock(start_block + block, bmapb)
		if err != nil {
			log.Printf("Unable to fetch bitmap block %d - %s", block, err)
			return NO_BIT
		}

		// Iterate over the words in a block
		for i := word; i < uint(len(bmapb)); i++ {
			num := bmapb[i]

			// Does this word contain a free bit?
			if num == math.MaxUint16 {
				// No bits free, move to next word
				continue
			}

			// Find and allocate the free bit
			var bit uint
			for bit = 0; (num & (1 << bit)) != 0; bit++ {
			}

			// Get the bit number from the start of the bit map
			b := (block * FS_BITS_PER_BLOCK(fs.Block_size)) + (i * FS_BITCHUNK_BITS) + bit

			// Don't allocate bits beyond the end of the map
			if b >= map_bits {
				break
			}

			// Allocate and return bit number
			num = num | (1 << bit)
			bmapb[i] = num

			// TODO: Make this block dirty
			fs.PutBlock(start_block + block, bmapb)
			return b
		}

		block = block + 1
		if (block) >= bit_blocks {
			block = 0
		}
		word = 0
		bcount = bcount - 1
		if bcount <= 0 {
			break
		}
	}

	return NO_BIT
}

// Deallocate an inode/zone in the bitmap, freeing it up for re-use
func (fs *FileSystem) FreeBit(bmap uint, bit_returned uint) {
	var start_block uint // first bit block

	if bmap == IMAP {
		start_block = START_BLOCK
	} else {
		start_block = START_BLOCK + fs.super.Imap_blocks
	}

	block := bit_returned / FS_BITS_PER_BLOCK(fs.Block_size)
	word := (bit_returned % FS_BITS_PER_BLOCK(fs.Block_size)) / FS_BITCHUNK_BITS

	bit := bit_returned % FS_BITCHUNK_BITS
	mask := uint16(1) << bit

	bmapb := make([]uint16, FS_BITMAP_CHUNKS(fs.Block_size))
	err := fs.GetBlock(start_block + block, bmapb)
	if err != nil {
		log.Printf("Unable to fetch bitmap block %d - %s", block, err)
		return
	}

	k := bmapb[word]
	if (k & mask) == 0 {
		if bmap == IMAP {
			panic("tried to free unused inode")
		} else if bmap == ZMAP {
			panic("tried to free unused block")
		}
	}

	k = k & (^ mask)
	bmapb[word] = k
	fs.PutBlock(start_block + block, bmapb)
}

