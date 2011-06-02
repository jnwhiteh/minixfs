package minixfs

// Remove all the zones from the inode and mark it as dirty
func (fs *FileSystem) truncate(rip *Inode) {
	file_type := rip.Mode & I_TYPE

	// check to see if the file is special
	if file_type == I_CHAR_SPECIAL || file_type == I_BLOCK_SPECIAL {
		return
	}

	super := fs.supers[rip.dev]
	scale := super.Log_zone_size
	zone_size := super.Block_size << scale
	nr_indirects := super.Block_size / V2_ZONE_NUM_SIZE

	// PIPE:
	// // Pipes can shrink, so adjust size to make sure all zones are removed
	// waspipe := rip.pipe
	// if waspipe {
	// 	rip.Size = PIPE_SIZE(fs.Block_size)
	// }

	// step through the file a zone at a time, finding and freeing the zones
	for position := uint(0); position < uint(rip.Size); position += zone_size {
		if b := fs.read_map(rip, position); b != NO_BLOCK {
			z := b >> scale
			fs.free_zone(rip.dev, z)
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
	fs.free_zone(rip.dev, uint(rip.Zone[single]))
	if z := rip.Zone[single+1]; z != NO_ZONE {
		// free all the single indirect zones pointed to by the double
		b := int(z << scale)
		bp := fs.get_block(rip.dev, b, INDIRECT_BLOCK, NORMAL)
		for i := uint(0); i < nr_indirects; i++ {
			z1 := fs.rd_indir(bp, i)
			fs.free_zone(rip.dev, z1)
		}
		// now free the double indirect zone itself
		fs.put_block(bp, INDIRECT_BLOCK)
		fs.free_zone(rip.dev, uint(z))
	}

	// leave zone numbers for de(1) to recover file after an unlink(2)
}
