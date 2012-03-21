package inode

import (
	"log"
	. "minixfs/common"
)

// Given an inode and a position within the corresponding file, locate the
// block (not zone) number in which that position is to be found and return
func ReadMap(inode *cacheInode, position int, cache BlockCache) int {
	devinfo := inode.devinfo

	scale := devinfo.Scale // for block-zone conversion
	blocksize := devinfo.Blocksize

	block_pos := position / blocksize            // relative block # in file
	zone := block_pos >> scale                   // position's zone
	boff := block_pos - (zone << scale)          // relative block in zone
	dzones := V2_NR_DZONES                       // number of direct zones
	nr_indirects := blocksize / V2_ZONE_NUM_SIZE // number of indirect zones

	// Is the position to be found in the inode itself?
	if zone < dzones {
		z := int(inode.Zone[zone])
		if z == NO_ZONE {
			return NO_BLOCK
		}
		b := (z << scale) + boff
		return b
	}

	// It is not in the inode, so must be single or double indirect
	var z int
	excess := zone - dzones

	if excess < nr_indirects {
		// 'position' can be located via the single indirect block
		z = int(inode.Zone[dzones])
	} else {
		// 'position' can be located via the double indirect block
		z = int(inode.Zone[dzones+1])
		if z == NO_ZONE {
			return NO_BLOCK
		}
		excess = excess - nr_indirects // single indirect doesn't count
		b := z << scale
		bp := cache.GetBlock(inode.devnum, int(b), INDIRECT_BLOCK, NORMAL) // get double indirect block
		index := excess / nr_indirects
		z = RdIndir(bp, index, cache, devinfo.Firstdatazone, devinfo.Zones) // z= zone for single
		cache.PutBlock(bp, INDIRECT_BLOCK)                                  // release double indirect block
		excess = excess % nr_indirects                                      // index into single indirect block
	}

	// 'z' is zone num for single indirect block; 'excess' is index into it
	if z == NO_ZONE {
		return NO_BLOCK
	}

	b := z << scale // b is block number for single indirect
	bp := cache.GetBlock(inode.devnum, int(b), INDIRECT_BLOCK, NORMAL)
	z = RdIndir(bp, excess, cache, devinfo.Firstdatazone, devinfo.Zones)
	cache.PutBlock(bp, INDIRECT_BLOCK)
	if z == NO_ZONE {
		return NO_BLOCK
	}
	b = (z << scale) + boff
	return b
}

// Given a pointer to an indirect block, read one entry with bounds checking
// on min/max.
func RdIndir(bp *CacheBlock, index int, cache BlockCache, min, max int) int {
	bpdata := bp.Block.(IndirectBlock)

	zone := int(bpdata[index])
	if zone != NO_ZONE && (zone < min || zone >= max) {
		log.Printf("Illegal zone number %ld in indirect block, index %d\n", zone, index)
		log.Printf("Firstdatazone_old: %d", min)
		log.Printf("Nzones: %d", max)
		panic("check file system")
	}
	return zone
}
