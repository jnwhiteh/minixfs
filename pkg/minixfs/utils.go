package minixfs


// This utility package assumes that each file system is a read/write file
// system and does not contain any mounted sub-filesystems. If either of
// these conditions is violated, the code will need to be adjusted.

// Allocate a bit from a bit map and return its bit number
// func (fs *FileSystem) AllocBit(bmap uint, origin uint32) (uint32) {
// 	var start_block uint16 // first bit block
// 	var map_bits uint32    // how many bits are there in the bit map
// 	var bit_blocks uint16 // how many blocks are there in the bit map
// 
// 	if bmap == IMAP {
// 		start_block = START_BLOCK
// 		map_bits = fs.super.Ninodes + 1
// 		bit_blocks = fs.super.Imap_blocks
// 	} else {
// 		start_block = START_BLOCK + fs.super.Imap_blocks
// 		map_bits = fs.super.Zones - uint32(fs.super.Firstdatazone_old-1)
// 		bit_blocks = fs.super.Zmap_blocks
// 	}
// 
// 	// Figure out where to start the bit search (depends on 'origin')
// 	if origin >= map_bits {
// 		origin = 0 // for robustness
// 	}
// 
// 	// Locate the starting place
// 	block := origin / FS_BITS_PER_BLOCK(fs.Block_size)
// 	word := (origin % FS_BITS_PER_BLOCK(fs.Block_size)) / FS_BITCHUNK_BITS
// 
// 	// Iterate over all blocks plus one, because we start in the middle
// 	bcount := bit_blocks + 1
// 	bmapb := make([]uint16, FS_BITMAP_CHUNKS(fs.Block_size))
// 	wlim := FS_BITMAP_CHUNKS(fs.Block_size)
// 
// 	for {
// 		err := fs.GetBlock(start_block + block, bmapb)
// 		if err != nil {
// 			log.Printf("Unable to fetch bitmap block %d - %s", block, err)
// 			return NO_BIT
// 		}
// 
// 		// Iterate over the words in a block
// 		for i := word; word = word + 1; word < wlim {
// 			num := bmapb[i]
// 
// 			// Does this word contain a free bit?
// 			if num == uint(~ 0) {
// 				continue
// 			}
// 		}
// 	}
// 
// 	if start_block == 0 || bit_blocks == 1 {
// 		// do something
// 	}
// 
// 	return 0
// }

// Allocate a free inode on the given FileSystem and return a pointer to it.
func (fs *FileSystem) AllocInode(mode uint16) (*Inode) {
	// Acquire an inode from the bit map
	return nil
}
