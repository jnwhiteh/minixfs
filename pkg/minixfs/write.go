package minixfs

import (
	"os"
)

// Acquire a new block and return a pointer to it. Doing so may require
// allocating a complete zone, and then returning the initial block. On the
// other hand, the current zone may still have some unused blocks.
func new_block(rip *CacheInode, position int, btype BlockType, cache BlockCache) (*CacheBlock, os.Error) {
	var b int
	var z int
	var err os.Error

	if b = read_map(rip, position, cache); b == NO_BLOCK {
		// Choose first zone if possible.
		// Lose if the file is non-empty but the first zone number is NO_ZONE,
		// corresponding to a zone full of zeros. It would be better to search
		// near the last real zone.
		if z, err = rip.super.AllocZone(int(rip.Zone(0))); z == NO_ZONE {
			return nil, err
		}
		if err = write_map(rip, position, z, cache); err != nil {
			rip.super.FreeZone(uint(z))
			return nil, err
		}

		// If we are not writing at EOF, clear the zone, just to be safe
		if position != int(rip.Size()) {
			clear_zone(rip, position, 1, cache)
		}
		scale := rip.Scale()
		base_block := z << scale
		zone_size := rip.BlockSize() << scale
		b = base_block + ((position % zone_size) / rip.BlockSize())
	}

	bp := cache.GetBlock(rip.dev, int(b), btype, NO_READ)
	zero_block(bp, btype, rip.BlockSize())
	return bp, nil
}

func zero_block(bp *CacheBlock, btype BlockType, blocksize int) {
	switch btype {
	case INODE_BLOCK:
		bp.block = make(InodeBlock, blocksize/V2_INODE_SIZE)
	case DIRECTORY_BLOCK:
		bp.block = make(DirectoryBlock, blocksize/V2_DIRENT_SIZE)
	case INDIRECT_BLOCK:
		bp.block = make(IndirectBlock, blocksize/4)
	case MAP_BLOCK:
		bp.block = make(MapBlock, blocksize/2)
	case FULL_DATA_BLOCK:
		bp.block = make(FullDataBlock, blocksize)
	case PARTIAL_DATA_BLOCK:
		bp.block = make(PartialDataBlock, blocksize)
	}
}

// Write a new zone into an inode
func write_map(rip *CacheInode, position int, new_zone int, cache BlockCache) os.Error {
	rip.SetDirty(true) // inode will be changed
	var bp *CacheBlock = nil
	var z int
	var z1 int
	var zindex int
	var err os.Error

	// relative zone # to insert
	zone := int((position / rip.BlockSize()) >> rip.Scale())
	zones := V2_NR_DZONES                                   // # direct zones in the inode
	nr_indirects := int(rip.BlockSize() / V2_ZONE_NUM_SIZE) // # indirect zones per indirect block

	// Is 'position' to be found in the inode itself?
	if zone < zones {
		zindex = zone
		rip.SetZone(zindex, uint32(new_zone))
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
		z1 = int(rip.Zone(zones)) // single indirect zone
		single = true
	} else {
		// 'position' can be located via the double indirect block
		if z = int(rip.Zone(zones + 1)); z == NO_ZONE {
			// Create the double indirect block
			z, err = rip.super.AllocZone(int(rip.Zone(0)))
			if z == NO_ZONE || err != nil {
				return err
			}
			rip.SetZone(zones+1, uint32(z))
			new_dbl = true
		}

		// Either way 'z' is a zone number for double indirect block
		excess -= nr_indirects // single indirect doesn't count
		ind_ex = excess / nr_indirects
		excess = excess % nr_indirects
		if ind_ex >= nr_indirects {
			return EFBIG
		}
		b := z << rip.Scale()
		var rdflag int
		if new_dbl {
			rdflag = NO_READ
		} else {
			rdflag = NORMAL
		}
		bp = cache.GetBlock(rip.dev, b, INDIRECT_BLOCK, rdflag)
		if new_dbl {
			zero_block(bp, INDIRECT_BLOCK, rip.BlockSize())
		}
		z1 = int(rd_indir(bp, ind_ex, cache, rip.Firstdatazone(), rip.Zones()))
		single = false
	}

	// z1 is now single indirect zone; 'excess' is index
	if z1 == NO_ZONE {
		// Create indirect block and store zone # in inode or dbl indir block
		z1, err = rip.super.AllocZone(int(rip.Zone(0)))
		if single {
			rip.SetZone(zones, uint32(z1)) // update inode
		} else {
			wr_indir(bp, ind_ex, z1) // update dbl indir
		}

		new_ind = true
		if bp != nil {
			bp.dirty = true // if double indirect, it is dirty
		}
		if z1 == NO_ZONE {
			cache.PutBlock(bp, INDIRECT_BLOCK) // release dbl indirect block
			return err                         // couldn't create single indirect block
		}
	}
	cache.PutBlock(bp, INDIRECT_BLOCK) // release dbl indirect block
	// z1 is indirect block's zone number
	b := z1 << rip.Scale()
	var rdflag int
	if new_dbl {
		rdflag = NO_READ
	} else {
		rdflag = NORMAL
	}
	bp = cache.GetBlock(rip.dev, b, INDIRECT_BLOCK, rdflag)
	if new_ind {
		zero_block(bp, INDIRECT_BLOCK, rip.BlockSize())
	}
	ex = excess
	wr_indir(bp, ex, int(new_zone))
	bp.dirty = true
	cache.PutBlock(bp, INDIRECT_BLOCK)
	return nil
}

// Zero a zone, possibly starting in the middle. The parameter 'pos' gives a
// byte in the first block to be zeroed. clear_zone is called from
// read_write() and new_block().
func clear_zone(rip *CacheInode, pos int, flag int, cache BlockCache) {
	scale := rip.Scale()

	// If the block size and zone size are the same, clear_zone not needed
	if scale == 0 {
		return
	}

	panic("Block size = zone size")

	zone_size := rip.BlockSize() << scale
	if flag == 1 {
		pos = (pos / zone_size) * zone_size
	}
	next := pos + rip.BlockSize() - 1

	// If 'pos' is in the last block of a zone, do not clear the zone
	if next/zone_size != pos/zone_size {
		return
	}

	blo := read_map(rip, next, cache)
	if blo == NO_BLOCK {
		return
	}
	bhi := (((blo >> scale) + 1) << scale) - 1

	// Clear all blocks between blo and bhi
	for b := blo; b < bhi; b++ {
		// TODO: I'm not sure the block type is correct here.
		bp := cache.GetBlock(rip.dev, int(b), FULL_DATA_BLOCK, NO_READ)
		zero_block(bp, FULL_DATA_BLOCK, rip.BlockSize())
		cache.PutBlock(bp, FULL_DATA_BLOCK)
	}
}

// Given a pointer to an indirect block, write one entry
func wr_indir(bp *CacheBlock, index int, zone int) {
	indb := bp.block.(IndirectBlock)
	indb[index] = uint32(zone)
}

// Write 'chunk' bytes from 'buff' into 'rip' at position 'pos' in the file.
// This is at offset 'off' within the current block.
func write_chunk(rip *CacheInode, pos, off, chunk int, buff []byte, cache BlockCache) os.Error {
	var bp *CacheBlock
	var err os.Error

	bsize := rip.BlockSize()
	fsize := int(rip.Size())
	b := read_map(rip, pos, cache)

	if b == NO_BLOCK {
		// Writing to a nonexistent block. Create and enter in inode
		bp, err = new_block(rip, pos, FULL_DATA_BLOCK, cache)
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
		bp = cache.GetBlock(rip.dev, int(b), FULL_DATA_BLOCK, n)
	}

	// In all cases, bp now points to a valid buffer
	if bp == nil {
		panic("bp not valid in rw_chunk, this can't happen")
	}

	if chunk != bsize && pos >= fsize && off == 0 {
		zero_block(bp, FULL_DATA_BLOCK, rip.BlockSize())
	}

	// Copy 'chunk' bytes from the user supplied buffer into the block
	// starting at 'off'.
	bdata := bp.block.(FullDataBlock)
	for i := 0; i < chunk; i++ {
		bdata[off+i] = buff[i]
	}

	bp.dirty = true

	if off+chunk == bsize {
		cache.PutBlock(bp, FULL_DATA_BLOCK)
	} else {
		cache.PutBlock(bp, PARTIAL_DATA_BLOCK)
	}

	return err
}
