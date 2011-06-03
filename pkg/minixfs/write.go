package minixfs

import (
	"os"
)

// Acquire a new block and return a pointer to it. Doing so may require
// allocating a complete zone, and then returning the initial block. On the
// other hand, the current zone may still have some unused blocks.
func (fs *FileSystem) new_block(rip *Inode, position uint, btype BlockType) (*buf, os.Error) {
	var b uint
	var z int
	var err os.Error

	if b = fs.read_map(rip, position); b == NO_BLOCK {
		// Choose first zone if possible.
		// Lose if the file is non-empty but the first zone number is NO_ZONE,
		// corresponding to a zone full of zeros. It would be better to search
		// near the last real zone.
		if rip.Zone[0] == NO_ZONE {
			z = int(fs.supers[rip.dev].Firstdatazone)
		} else {
			z = int(rip.Zone[0])
		}
		if z, err = fs.alloc_zone(rip.dev, z); z == NO_ZONE {
			return nil, err
		}
		if err = fs.write_map(rip, position, uint(z)); err != nil {
			fs.free_zone(rip.dev, uint(z))
			return nil, err
		}

		// If we are not writing at EOF, clear the zone, just to be safe
		if position != uint(rip.Size) {
			fs.clear_zone(rip, position, 1)
		}
		scale := fs.supers[rip.dev].Log_zone_size
		base_block := z << scale
		zone_size := fs.supers[rip.dev].Block_size << scale
		b = uint(base_block) + ((position % zone_size) / fs.supers[rip.dev].Block_size)
	}

	bp := fs.get_block(rip.dev, int(b), btype, NO_READ)
	fs.zero_block(bp, btype)
	return bp, nil
}

func (fs *FileSystem) zero_block(bp *buf, btype BlockType) {
	blocksize := fs.supers[bp.dev].Block_size
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
func (fs *FileSystem) write_map(rip *Inode, position uint, new_zone uint) os.Error {
	rip.dirty = true // inode will be changed
	var bp *buf = nil
	var z int
	var z1 int
	var zindex int
	var err os.Error

	sp := fs.supers[rip.dev]

	scale := sp.Log_zone_size // for zone-block voncersion
	// relative zone # to insert
	zone := int((position / sp.Block_size) >> scale)
	zones := V2_NR_DZONES                                 // # direct zones in the inode
	nr_indirects := int(sp.Block_size / V2_ZONE_NUM_SIZE) // # indirect zones per indirect block

	// Is 'position' to be found in the inode itself?
	if zone < zones {
		zindex = zone
		rip.Zone[zindex] = uint32(new_zone)
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
		z1 = int(rip.Zone[zones]) // single indirect zone
		single = true
	} else {
		// 'position' can be located via the double indirect block
		if z = int(rip.Zone[zones+1]); z == NO_ZONE {
			// Create the double indirect block
			z, err := fs.alloc_zone(rip.dev, int(rip.Zone[0]))
			if z == NO_ZONE || err != nil {
				return err
			}
			rip.Zone[zones+1] = uint32(z)
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
		bp = fs.get_block(rip.dev, b, INDIRECT_BLOCK, rdflag)
		if new_dbl {
			fs.zero_block(bp, INDIRECT_BLOCK)
		}
		z1 = int(fs.rd_indir(bp, uint(ind_ex)))
		single = false
	}

	// z1 is now single indirect zone; 'excess' is index
	if z1 == NO_ZONE {
		// Create indirect block and store zone # in inode or dbl indir block
		z1, err = fs.alloc_zone(rip.dev, int(rip.Zone[0]))
		if single {
			rip.Zone[zones] = uint32(z1) // update inode
		} else {
			fs.wr_indir(bp, ind_ex, z1) // update dbl indir
		}

		new_ind = true
		if bp != nil {
			bp.dirty = true // if double indirect, it is dirty
		}
		if z1 == NO_ZONE {
			fs.put_block(bp, INDIRECT_BLOCK) // release dbl indirect block
			return err                       // couldn't create single indirect block
		}
	}
	fs.put_block(bp, INDIRECT_BLOCK) // release dbl indirect block
	// z1 is indirect block's zone number
	b := z1 << scale
	var rdflag int
	if new_dbl {
		rdflag = NO_READ
	} else {
		rdflag = NORMAL
	}
	bp = fs.get_block(rip.dev, b, INDIRECT_BLOCK, rdflag)
	if new_ind {
		fs.zero_block(bp, INDIRECT_BLOCK)
	}
	ex = excess
	fs.wr_indir(bp, ex, int(new_zone))
	bp.dirty = true
	fs.put_block(bp, INDIRECT_BLOCK)
	return nil
}

// Zero a zone, possibly starting in the middle. The parameter 'pos' gives a
// byte in the first block to be zeroed. clear_zone is called from
// read_write() and new_block().
func (fs *FileSystem) clear_zone(rip *Inode, pos uint, flag int) {
	sp := fs.supers[rip.dev]
	scale := sp.Log_zone_size

	// If the block size and zone size are the same, clear_zone not needed
	if scale == 0 {
		return
	}

	panic("Block size = zone size")

	zone_size := sp.Block_size << scale
	if flag == 1 {
		pos = (pos / zone_size) * zone_size
	}
	next := pos + sp.Block_size - 1

	// If 'pos' is in the last block of a zone, do not clear the zone
	if next/zone_size != pos/zone_size {
		return
	}
	blo := fs.read_map(rip, next)
	if blo == NO_BLOCK {
		return
	}
	bhi := (((blo >> scale) + 1) << scale) - 1

	// Clear all blocks between blo and bhi
	for b := blo; b < bhi; b++ {
		// TODO: I'm not sure the block type is correct here.
		bp := fs.get_block(rip.dev, int(b), FULL_DATA_BLOCK, NO_READ)
		fs.zero_block(bp, FULL_DATA_BLOCK)
		fs.put_block(bp, FULL_DATA_BLOCK)
	}
}

// Given a pointer to an indirect block, write one entry
func (fs *FileSystem) wr_indir(bp *buf, index int, zone int) {
	indb := bp.block.(IndirectBlock)
	indb[index] = uint32(zone)
}
