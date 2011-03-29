package minixfs

import "log"

// Remove all the zones from the inode and mark it as dirty
func (fs *FileSystem) Truncate(rip *Inode) {
	file_type := rip.Mode & I_TYPE

	// check to see if the file is special
	if file_type == I_CHAR_SPECIAL || file_type == I_BLOCK_SPECIAL {
		return
	}

	scale := fs.Log_zone_size
	zone_size := fs.Block_size << scale
	nr_indirects := fs.super.Block_size / V2_ZONE_NUM_SIZE

	// PIPE:
	// // Pipes can shrink, so adjust size to make sure all zones are removed
	// waspipe := rip.pipe
	// if waspipe {
	// 	rip.Size = PIPE_SIZE(fs.Block_size)
	// }

	// step through the file a zone at a time, finding and freeing the zones
	for position := uint(0); position < uint(rip.Size); position += zone_size {
		if b := fs.ReadMap(rip, position); b != NO_BLOCK {
			z := b >> scale
			fs.FreeZone(z)
		}
	}

	// all the dirty zones have been freed. Now free the indirect zones
	rip.dirty = true
	// PIPE:
	// if waspipe {
	// 	fs.WipeInode(rip)
	// 	return
	// }
	single := V2_NR_DZONES
	fs.FreeZone(uint(rip.Zone[single]))
	if z := rip.Zone[single+1]; z != NO_ZONE {
		// free all the single indirect zones pointed to by the double
		b := uint(z << scale)
		bp, err := fs.GetIndirectBlock(b)
		if err != nil {
			log.Printf("Failed when fetching indirect block %d", b)
			panic("Failed to truncate file")
		}
		for i := uint(0); i < nr_indirects; i++ {
			z1 := fs.RdIndir(bp, i)
			fs.FreeZone(z1)
		}
		// now free the double indirect zone itself
		fs.PutBlock(bp, INDIRECT_BLOCK)
		fs.FreeZone(uint(z))
	}

	// leave zone numbers for de(1) to recover file after an unlink(2)
}
