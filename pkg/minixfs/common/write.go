package common

import (
	"os"
)

// Zero a zone, possibly starting in the middle. The parameter 'pos' gives a
// byte in the first block to be zeroed. clear_zone is called from
// read_write() and new_block().
func ClearZone(rip *CacheInode, pos int, flag int, cache BlockCache) {
	scale := rip.Devinfo.Scale
	blocksize := rip.Devinfo.Blocksize

	// If the block size and zone size are the same, clear_zone not needed
	if scale == 0 {
		return
	}

	panic("Block size = zone size")

	zone_size := blocksize << scale
	if flag == 1 {
		pos = (pos / zone_size) * zone_size
	}
	next := pos + blocksize - 1

	// If 'pos' is in the last block of a zone, do not clear the zone
	if next/zone_size != pos/zone_size {
		return
	}

	blo := ReadMap(rip, next, cache)
	if blo == NO_BLOCK {
		return
	}
	bhi := (((blo >> scale) + 1) << scale) - 1

	// Clear all blocks between blo and bhi
	for b := blo; b < bhi; b++ {
		// TODO: I'm not sure the block type is correct here.
		bp := cache.GetBlock(rip.Devno, int(b), FULL_DATA_BLOCK, NO_READ)
		ZeroBlock(bp, FULL_DATA_BLOCK, blocksize)
		cache.PutBlock(bp, FULL_DATA_BLOCK)
	}
}

func ZeroBlock(bp *CacheBlock, btype BlockType, blocksize int) {
	switch btype {
	case INODE_BLOCK:
		bp.Block = make(InodeBlock, blocksize/V2_INODE_SIZE)
	case DIRECTORY_BLOCK:
		bp.Block = make(DirectoryBlock, blocksize/V2_DIRENT_SIZE)
	case INDIRECT_BLOCK:
		bp.Block = make(IndirectBlock, blocksize/4)
	case MAP_BLOCK:
		bp.Block = make(MapBlock, blocksize/2)
	case FULL_DATA_BLOCK:
		bp.Block = make(FullDataBlock, blocksize)
	case PARTIAL_DATA_BLOCK:
		bp.Block = make(PartialDataBlock, blocksize)
	}
}

// Write 'chunk' bytes from 'buff' into 'rip' at position 'pos' in the file.
// This is at offset 'off' within the current block.
func WriteChunk(rip *CacheInode, pos, off, chunk int, buff []byte, cache BlockCache) os.Error {
	var bp *CacheBlock
	var err os.Error

	bsize := rip.Devinfo.Blocksize
	fsize := int(rip.Inode.Size)
	b := ReadMap(rip, pos, cache)

	if b == NO_BLOCK {
		// Writing to a nonexistent block. Create and enter in inode
		bp, err = NewBlock(rip, pos, FULL_DATA_BLOCK, cache)
		if bp == nil || err != nil {
			return err
		}
	} else {
		// Normally an existing block to be parially overwritten is first read
		// in. However, a full block need not be read in. If it is already in
		// the cache, acquire it, otherwise just acquire a free buffer.
		n := NORMAL
		if chunk == bsize {
			n = NO_READ
		}
		if off == 0 && pos >= fsize {
			n = NO_READ
		}
		bp = cache.GetBlock(rip.Devno, int(b), FULL_DATA_BLOCK, n)
	}

	// In all cases, bp now points to a valid buffer
	if bp == nil {
		panic("bp not valid in rw_chunk, this can't happen")
	}

	if chunk != bsize && pos >= fsize && off == 0 {
		ZeroBlock(bp, FULL_DATA_BLOCK, rip.Devinfo.Blocksize)
	}

	// Copy 'chunk' bytes from the user supplied buffer into the block
	// starting at 'off'.
	bdata := bp.Block.(FullDataBlock)
	for i := 0; i < chunk; i++ {
		bdata[off+i] = buff[i]
	}

	bp.Dirty = true

	if off+chunk == bsize {
		cache.PutBlock(bp, FULL_DATA_BLOCK)
	} else {
		cache.PutBlock(bp, PARTIAL_DATA_BLOCK)
	}

	return err
}

// Acquire a new block and return a pointer to it. Doing so may require
// allocating a complete zone, and then returning the initial block. On the
// other hand, the current zone may still have some unused blocks.
func NewBlock(rip *CacheInode, position int, btype BlockType, cache BlockCache) (*CacheBlock, os.Error) {
	var b int
	var z int
	var err os.Error

	if b = ReadMap(rip, position, cache); b == NO_BLOCK {
		// Choose first zone if possible.
		// Lose if the file is non-empty but the first zone number is NO_ZONE,
		// corresponding to a zone full of zeros. It would be better to search
		// near the last real zone.
		if z, err = rip.Bitmap.AllocZone(int(rip.Inode.Zone[0])); z == NO_ZONE {
			return nil, err
		}
		if err = WriteMap(rip, position, z, cache); err != nil {
			rip.Bitmap.FreeZone(z)
			return nil, err
		}

		// If we are not writing at EOF, clear the zone, just to be safe
		if position != int(rip.Inode.Size) {
			ClearZone(rip, position, 1, cache)
		}
		scale := rip.Devinfo.Scale
		blocksize := rip.Devinfo.Blocksize
		base_block := z << scale
		zone_size := blocksize << scale
		b = base_block + ((position % zone_size) / blocksize)
	}

	bp := cache.GetBlock(rip.Devno, int(b), btype, NO_READ)
	ZeroBlock(bp, btype, rip.Devinfo.Blocksize)
	return bp, nil
}

// Write a new zone into an inode
func WriteMap(rip *CacheInode, position int, new_zone int, cache BlockCache) os.Error {
	rip.Dirty = true // inode will be changed
	var bp *CacheBlock = nil
	var z int
	var z1 int
	var zindex int
	var err os.Error

	// relative zone # to insert
	blocksize := rip.Devinfo.Blocksize
	scale := rip.Devinfo.Scale

	zone := int((position / blocksize) >> scale)
	zones := V2_NR_DZONES                             // # direct zones in the inode
	nr_indirects := int(blocksize / V2_ZONE_NUM_SIZE) // # indirect zones per indirect block

	// Is 'position' to be found in the inode itself?
	if zone < zones {
		zindex = zone
		rip.Inode.Zone[zindex] = uint32(new_zone)
		return nil
	}

	// It is not in the inode, so it must be in the single or double indirect
	var ind_ex int
	var ex int
	excess := zone - zones // first V2_NR_DZONES don't count
	new_ind := false
	new_dbl := false
	single := true

	if excess < int(nr_indirects) {
		// 'position' can be located via the single indirect block
		z1 = int(rip.Inode.Zone[zones]) // single indirect zone
		single = true
	} else {
		// 'position' can be located via the double indirect block
		if z = int(rip.Inode.Zone[zones+1]); z == NO_ZONE {
			// Create the double indirect block
			z, err = rip.Bitmap.AllocZone(int(rip.Inode.Zone[0]))
			if z == NO_ZONE || err != nil {
				return err
			}
			rip.Inode.Zone[zones+1] = uint32(z)
			new_dbl = true
		}

		// Either way 'z' is a zone number for double indirect block
		excess -= nr_indirects // single indirect doesn't count
		ind_ex = excess / nr_indirects
		excess = excess % nr_indirects
		if ind_ex >= nr_indirects {
			return EFBIG
		}
		b := z << scale
		var rdflag int
		if new_dbl {
			rdflag = NO_READ
		} else {
			rdflag = NORMAL
		}
		bp = cache.GetBlock(rip.Devno, b, INDIRECT_BLOCK, rdflag)
		if new_dbl {
			ZeroBlock(bp, INDIRECT_BLOCK, blocksize)
		}
		z1 = int(RdIndir(bp, ind_ex, cache, rip.Devinfo.Firstdatazone, rip.Devinfo.Zones))
		single = false
	}

	// z1 is now single indirect zone; 'excess' is index
	if z1 == NO_ZONE {
		// Create indirect block and store zone # in inode or dbl indir block
		z1, err = rip.Bitmap.AllocZone(int(rip.Inode.Zone[0]))
		if single {
			rip.Inode.Zone[zones] = uint32(z1) // update inode
		} else {
			WrIndir(bp, ind_ex, z1) // update dbl indir
		}

		new_ind = true
		if bp != nil {
			bp.Dirty = true // if double indirect, it is dirty
		}
		if z1 == NO_ZONE {
			cache.PutBlock(bp, INDIRECT_BLOCK) // release dbl indirect block
			return err                         // couldn't create single indirect block
		}
	}
	cache.PutBlock(bp, INDIRECT_BLOCK) // release dbl indirect block
	// z1 is indirect block's zone number
	b := z1 << scale
	var rdflag int
	if new_dbl {
		rdflag = NO_READ
	} else {
		rdflag = NORMAL
	}
	bp = cache.GetBlock(rip.Devno, b, INDIRECT_BLOCK, rdflag)
	if new_ind {
		ZeroBlock(bp, INDIRECT_BLOCK, blocksize)
	}
	ex = excess
	WrIndir(bp, ex, int(new_zone))
	bp.Dirty = true
	cache.PutBlock(bp, INDIRECT_BLOCK)
	return nil
}

// Given a pointer to an indirect block, write one entry
func WrIndir(bp *CacheBlock, index int, zone int) {
	indb := bp.Block.(IndirectBlock)
	indb[index] = uint32(zone)
}

// Remove all the zones from the inode and mark it as dirty
func Truncate(rip *CacheInode, bmap Bitmap, cache BlockCache) {
	ftype := rip.Inode.Mode & I_TYPE

	// check to see if the file is special
	if ftype == I_CHAR_SPECIAL || ftype == I_BLOCK_SPECIAL {
		return
	}

	scale := rip.Devinfo.Scale
	zone_size := rip.Devinfo.Blocksize << scale
	nr_indirects := rip.Devinfo.Blocksize / V2_ZONE_NUM_SIZE

	// PIPE:
	// // Pipes can shrink, so adjust size to make sure all zones are removed
	// waspipe := rip.pipe
	// if waspipe {
	// 	rip.Size = PIPE_SIZE(fs.Block_size)
	// }

	// step through the file a zone at a time, finding and freeing the zones
	for position := 0; position < int(rip.Inode.Size); position += zone_size {
		if b := ReadMap(rip, position, cache); b != NO_BLOCK {
			z := b >> scale
			bmap.FreeZone(z)
		}
	}

	// all the dirty zones have been freed. Now free the indirect zones
	rip.Dirty = true
	// PIPE:
	// if waspipe {
	// 	fs.WipeInode(rip)
	// 	return
	// }
	single := V2_NR_DZONES
	bmap.FreeZone(int(rip.Inode.Zone[single]))

	if z := int(rip.Inode.Zone[single+1]); z != NO_ZONE {
		// free all the single indirect zones pointed to by the double
		b := int(z << scale)
		bp := cache.GetBlock(rip.Devno, b, INDIRECT_BLOCK, NORMAL)
		for i := 0; i < nr_indirects; i++ {
			z1 := RdIndir(bp, i, cache, rip.Devinfo.Firstdatazone, rip.Devinfo.Zones)
			bmap.FreeZone(z1)
		}
		// now free the double indirect zone itself
		cache.PutBlock(bp, INDIRECT_BLOCK)
		bmap.FreeZone(z)
	}

	// leave zone numbers for de(1) to recover file after an unlink(2)
}
