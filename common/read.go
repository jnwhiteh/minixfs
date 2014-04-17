package common

import (
	"io"
	"log"
)

// Given an inode and a position within the corresponding file, locate the
// block (not zone) number in which that position is to be found and return
func ReadMap(rip *Inode, position int, cache BlockCache) int {
	devinfo := rip.Devinfo
	zmap := rip.Zone
	scale := devinfo.Scale // for block-zone conversion
	blocksize := devinfo.Blocksize
	devnum := devinfo.Devnum

	block_pos := position / blocksize            // relative block # in file
	zone := block_pos >> scale                   // position's zone
	boff := block_pos - (zone << scale)          // relative block in zone
	dzones := V2_NR_DZONES                       // number of direct zones
	nr_indirects := blocksize / V2_ZONE_NUM_SIZE // number of indirect zones

	// Is the position to be found in the inode itself?
	if zone < dzones {
		z := int(zmap[zone])
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
		z = int(zmap[dzones])
	} else {
		// 'position' can be located via the double indirect block
		z = int(zmap[dzones+1])
		if z == NO_ZONE {
			return NO_BLOCK
		}
		excess = excess - nr_indirects // single indirect doesn't count
		b := z << scale
		bp := cache.GetBlock(devnum, int(b), INDIRECT_BLOCK, NORMAL) // get double indirect block
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
	bp := cache.GetBlock(devnum, int(b), INDIRECT_BLOCK, NORMAL)
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

func Read(rip *Inode, b []byte, pos int) (int, error) {
	devinfo := rip.Devinfo

	// We want to read at most len(b) bytes from the given inode. This data
	// will almost certainly be split up amongst multiple blocks.
	curpos := pos

	// Rather than getting fancy, just slice b to contain only enough space
	// for the data that is available
	// TODO: Should this rely on the inode size?
	if curpos+len(b) > int(rip.Size) {
		b = b[:int(rip.Size)-curpos]
	}

	if curpos >= int(rip.Size) {
		return 0, io.EOF
	}

	blocksize := devinfo.Blocksize

	// We can't just start reading at the start of a block, since we may be at
	// an offset within that block. So work out the chunk to read
	offset := curpos % blocksize
	bnum := ReadMap(rip, curpos, rip.Bcache)

	// TODO: Error check this
	// read the data block and copy the portion of data we need
	bp := rip.Bcache.GetBlock(devinfo.Devnum, bnum, FULL_DATA_BLOCK, NORMAL)
	bdata, bok := bp.Block.(FullDataBlock)
	if !bok {
		// TODO: Attempt to read from an invalid location, what should happen?
		return 0, EINVAL
	}

	if len(b) < blocksize-offset { // this block contains all the data we need
		for i := 0; i < len(b); i++ {
			b[i] = bdata[offset+i]
		}
		curpos += len(b)
		rip.Bcache.PutBlock(bp, FULL_DATA_BLOCK)
		return len(b), nil
	}

	// we need this entire block, so start riplling
	var numBytes int = 0
	for i := 0; i < blocksize-offset; i++ {
		b[i] = bdata[offset+i]
		numBytes++
	}

	rip.Bcache.PutBlock(bp, FULL_DATA_BLOCK)
	curpos += numBytes

	// At this stage, all reads should be on block boundaries. The ripnal block
	// will likely be a partial block, so handle that specially.
	for numBytes < len(b) {
		bnum = ReadMap(rip, curpos, rip.Bcache)
		bp := rip.Bcache.GetBlock(devinfo.Devnum, bnum, FULL_DATA_BLOCK, NORMAL)
		if _, sok := bp.Block.(FullDataBlock); !sok {
			log.Printf("block num: %d", bp.Blocknum)
			log.Panicf("When reading block %d for position %d, got IndirectBlock", bnum, curpos)
		}

		bdata = bp.Block.(FullDataBlock)

		bytesLeft := len(b) - numBytes // the number of bytes still needed

		// If we only need a portion of this block
		if bytesLeft < blocksize {

			for i := 0; i < bytesLeft; i++ {
				b[numBytes] = bdata[i]
				numBytes++
			}

			curpos += bytesLeft
			rip.Bcache.PutBlock(bp, FULL_DATA_BLOCK)
			return numBytes, nil
		}

		// We need this whole block
		for i := 0; i < len(bdata); i++ {
			b[numBytes] = bdata[i]
			numBytes++
		}

		curpos += len(bdata)
		rip.Bcache.PutBlock(bp, FULL_DATA_BLOCK)
	}

	return numBytes, nil
}
