package minixfs

import (
	"log"
	"os"
	"sync"
)

// A Finode is a process-oriented file inode, shared amongst all open
// instances of the file represented by this inode. File system operations
// that do not affect this file must not block a read() call to this file.
// Additionally, no read operations on this file should block an independent
// read() call for this file. In particular, open/close operations must not
// block reads, and multiple independent read requests must be allowed.
type Finode struct {
	fs        _FileSystem
	inode     *Inode
	scale     uint
	blocksize int
	maxsize   int
	cache     BlockCache
	count     int

	in  chan m_finode_req
	out chan m_finode_res

	waitGroup *sync.WaitGroup // used for mutual exclusion for writes
	closed    chan bool
}

func (fi *Finode) loop() {
	var in <-chan m_finode_req = fi.in
	var out chan<- m_finode_res = fi.out
	for req := range in {
		switch req := req.(type) {
		case m_finode_req_read:
			fi.waitGroup.Add(1)

			callback := make(chan m_finode_res_io)
			out <- m_finode_res_asyncio{callback}

			// Launch a new goroutine to perform the read, using the callback
			// channel to return the result.
			go func() {
				defer close(callback)
				defer fi.waitGroup.Done()
				n, err := fi.read(req.buf, req.pos)
				callback <- m_finode_res_io{n, err}
			}()
		case m_finode_req_write:
			// Wait for any outstanding read requests to finish
			fi.waitGroup.Wait()

			n, err := fi.write(req.buf, req.pos)
			out <- m_finode_res_io{n, err}
		case m_finode_req_close:
			fi.waitGroup.Wait()
			out <- m_finode_res_empty{}
			close(fi.in)
			close(fi.out)
		}
	}

	if fi.closed != nil {
		fi.closed <- true
	}
}

// Read up to len(b) bytes from the file from position 'pos'
func (fi *Finode) Read(b []byte, pos int) (int, os.Error) {
	fi.in <- m_finode_req_read{b, pos}
	ares := (<-fi.out).(m_finode_res_asyncio)
	res := (<-ares.callback)
	return res.n, res.err
}

// Write len(b) bytes to the file at position 'pos'
func (fi *Finode) Write(data []byte, pos int) (n int, err os.Error) {
	fi.in <- m_finode_req_write{data, pos}
	res := (<-fi.out).(m_finode_res_io)
	return res.n, res.err
}

// Close an instance of this finode.
func (fi *Finode) Close() {
	fi.in <- m_finode_req_close{}
	<-fi.out

	// this fails
	// res := (<-fi.out).(m_finode_res_empty)
	// _ = res // dummy

	return
}

func (fi *Finode) read(b []byte, pos int) (int, os.Error) {
	// We want to read at most len(b) bytes from the given file. This data
	// will almost certainly be split up amongst multiple blocks.
	curpos := pos

	// Determine what the ending position to be read is
	endpos := curpos + len(b)
	fsize := int(fi.inode.Size())
	if endpos >= int(fsize) {
		endpos = int(fsize) - 1
	}

	blocksize := fi.blocksize

	// We can't just start reading at the start of a block, since we may be at
	// an offset within that block. So work out the first chunk to read
	offset := curpos % blocksize
	bnum := read_map(fi.inode, curpos, fi.cache)

	// TODO: Error check this
	// read the first data block and copy the portion of data we need
	bp := fi.cache.GetBlock(fi.inode.dev, bnum, FULL_DATA_BLOCK, NORMAL)
	bdata, bok := bp.block.(FullDataBlock)
	if !bok {
		// TODO: Attempt to read from an invalid location, what should happen?
		return 0, EINVAL
	}

	if len(b) < blocksize-offset { // this block contains all the data we need
		for i := 0; i < len(b); i++ {
			b[i] = bdata[offset+i]
		}
		curpos += len(b)
		fi.cache.PutBlock(bp, FULL_DATA_BLOCK)
		return len(b), nil
	}

	// we need this entire first block, so start filling
	var numBytes int = 0
	for i := 0; i < blocksize-offset; i++ {
		b[i] = bdata[offset+i]
		numBytes++
	}

	fi.cache.PutBlock(bp, FULL_DATA_BLOCK)
	curpos += numBytes

	// At this stage, all reads should be on block boundaries. The final block
	// will likely be a partial block, so handle that specially.
	for numBytes < len(b) {
		bnum = read_map(fi.inode, curpos, fi.cache)
		bp := fi.cache.GetBlock(fi.inode.dev, bnum, FULL_DATA_BLOCK, NORMAL)
		if _, sok := bp.block.(FullDataBlock); !sok {
			log.Printf("block num: %d, count: %d", bp.blocknr, bp.count)
			log.Panicf("When reading block %d for position %d, got IndirectBlock", bnum, curpos)
		}

		bdata = bp.block.(FullDataBlock)

		bytesLeft := len(b) - numBytes // the number of bytes still needed

		// If we only need a portion of this block
		if bytesLeft < blocksize {

			for i := 0; i < bytesLeft; i++ {
				b[numBytes] = bdata[i]
				numBytes++
			}

			curpos += bytesLeft
			fi.cache.PutBlock(bp, FULL_DATA_BLOCK)
			return numBytes, nil
		}

		// We need this whole block
		for i := 0; i < len(bdata); i++ {
			b[numBytes] = bdata[i]
			numBytes++
		}

		curpos += len(bdata)
		fi.cache.PutBlock(bp, FULL_DATA_BLOCK)
	}

	return numBytes, nil
}

func (fi *Finode) write(data []byte, pos int) (n int, err os.Error) {
	// TODO: This implementation is direct and doesn't match the abstractions
	// in the original source. At some point it should be reviewed.
	cum_io := 0
	position := pos
	fsize := int(fi.inode.Size())

	// Check in advance to see if file will grow too big
	if position > fi.maxsize-len(data) {
		return 0, EFBIG
	}

	// Clear the zone containing the current present EOF if hole about to be
	// created. This is necessary because all unwritten blocks prior to the
	// EOF must read as zeros.
	if position > fsize {
		clear_zone(fi, fsize, 0)
	}

	bsize := fi.blocksize
	nbytes := len(data)
	// Split the transfer into chunks that don't span two blocks.
	for nbytes != 0 {
		off := (position % bsize)
		chunk := _MIN(nbytes, bsize-off)
		if chunk < 0 {
			chunk = bsize - off
		}

		// Read or write 'chunk' bytes, fetch the first block
		err = write_chunk(fi, position, off, chunk, data)
		if err != nil {
			break // EOF reached
		}

		// Update counters and pointers
		data = data[chunk:] // user buffer
		nbytes -= chunk     // bytes yet to be written
		cum_io += chunk     // bytes written so far
		position += chunk   // position within the file
	}

	if fi.inode.GetType() == I_REGULAR || fi.inode.GetType() == I_DIRECTORY {
		if position > fsize {
			fi.inode.SetSize(int32(position))
		}
	}

	// TODO: Update times
	if err == nil {
		fi.inode.SetDirty(true)
	}

	return cum_io, err
}

// Given a pointer to an indirect block, read one entry.
func rd_indir(bp *CacheBlock, index int, cache BlockCache) int {
	bpdata := bp.block.(IndirectBlock)

	zone := int(bpdata[index])
	// TODO: Re-establish this error checking
	// if zone != NO_ZONE && (zone < super.Firstdatazone || zone >= super.Zones) {
	// 	log.Printf("Illegal zone number %ld in indirect block, index %d\n", zone, index)
	// 	log.Printf("Firstdatazone_old: %d", super.Firstdatazone)
	// 	log.Printf("Nzones: %d", super.Nzones)
	// 	panic("check file system")
	// }
	return zone
}

// Zero a zone, possibly starting in the middle. The parameter 'pos' gives a
// byte in the first block to be zeroed. clear_zone is called from
// read_write() and new_block().
func clear_zone(fi *Finode, pos int, flag int) {
	scale := fi.scale

	// If the block size and zone size are the same, clear_zone not needed
	if scale == 0 {
		return
	}

	panic("Block size != zone size")
}

// Write 'chunk' bytes from 'buff' into 'rip' at position 'pos' in the file.
// This is at offset 'off' within the current block.
func write_chunk(fi *Finode, pos, off, chunk int, buff []byte) os.Error {
	var bp *CacheBlock
	var err os.Error

	bsize := fi.blocksize
	fsize := int(fi.inode.Size())
	b := read_map(fi.inode, pos, fi.cache)

	if b == NO_BLOCK {
		// Writing to a nonexistent block. Create and enter in inode
		bp, err = new_block(fi, pos, FULL_DATA_BLOCK)
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
		bp = fi.cache.GetBlock(fi.inode.dev, int(b), FULL_DATA_BLOCK, n)
	}

	// In all cases, bp now points to a valid buffer
	if bp == nil {
		panic("bp not valid in rw_chunk, this can't happen")
	}

	if chunk != bsize && pos >= fsize && off == 0 {
		zero_block(bp, FULL_DATA_BLOCK, fi.blocksize)
	}

	// Copy 'chunk' bytes from the user supplied buffer into the block
	// starting at 'off'.
	bdata := bp.block.(FullDataBlock)
	for i := 0; i < chunk; i++ {
		bdata[off+i] = buff[i]
	}

	bp.dirty = true

	if off+chunk == bsize {
		fi.cache.PutBlock(bp, FULL_DATA_BLOCK)
	} else {
		fi.cache.PutBlock(bp, PARTIAL_DATA_BLOCK)
	}

	return err
}

// Acquire a new block and return a pointer to it. Doing so may require
// allocating a complete zone, and then returning the initial block. On the
// other hand, the current zone may still have some unused blocks.
func new_block(fi *Finode, position int, btype BlockType) (*CacheBlock, os.Error) {
	var b int
	var z int
	var err os.Error

	rip := fi.inode

	if b = read_map(fi.inode, position, fi.cache); b == NO_BLOCK {
		// Choose first zone if possible.
		// Lose if the file is non-empty but the first zone number is NO_ZONE,
		// corresponding to a zone full of zeros. It would be better to search
		// near the last real zone.
		if z, err = fi.fs._AllocZone(rip.dev, int(rip.Zone(0))); z == NO_ZONE {
			return nil, err
		}
		if err = write_map(fi, position, z); err != nil {
			fi.fs._FreeZone(rip.dev, z)
			return nil, err
		}

		// If we are not writing at EOF, clear the zone, just to be safe
		if position != int(rip.Size()) {
			clear_zone(fi, position, 1)
		}
		scale := fi.scale
		base_block := z << scale
		zone_size := fi.blocksize << scale
		b = base_block + ((position % zone_size) / fi.blocksize)
	}

	bp := fi.cache.GetBlock(rip.dev, int(b), btype, NO_READ)
	zero_block(bp, btype, fi.blocksize)
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
func write_map(fi *Finode, position int, new_zone int) os.Error {
	rip := fi.inode

	rip.SetDirty(true) // inode will be changed
	var bp *CacheBlock = nil
	var z int
	var z1 int
	var zindex int
	var err os.Error

	// relative zone # to insert
	zone := int((position / fi.blocksize) >> fi.scale)
	zones := V2_NR_DZONES                                // # direct zones in the inode
	nr_indirects := int(fi.blocksize / V2_ZONE_NUM_SIZE) // # indirect zones per indirect block

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
			z, err = fi.fs._AllocZone(rip.dev, int(rip.Zone(0)))
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
		b := z << fi.scale
		var rdflag int
		if new_dbl {
			rdflag = NO_READ
		} else {
			rdflag = NORMAL
		}
		bp = fi.cache.GetBlock(rip.dev, b, INDIRECT_BLOCK, rdflag)
		if new_dbl {
			zero_block(bp, INDIRECT_BLOCK, fi.blocksize)
		}
		z1 = int(rd_indir(bp, ind_ex, fi.cache))
		single = false
	}

	// z1 is now single indirect zone; 'excess' is index
	if z1 == NO_ZONE {
		// Create indirect block and store zone # in inode or dbl indir block
		z1, err = fi.fs._AllocZone(rip.dev, int(rip.Zone(0)))
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
			fi.cache.PutBlock(bp, INDIRECT_BLOCK) // release dbl indirect block
			return err                            // couldn't create single indirect block
		}
	}
	fi.cache.PutBlock(bp, INDIRECT_BLOCK) // release dbl indirect block
	// z1 is indirect block's zone number
	b := z1 << fi.scale
	var rdflag int
	if new_dbl {
		rdflag = NO_READ
	} else {
		rdflag = NORMAL
	}
	bp = fi.cache.GetBlock(rip.dev, b, INDIRECT_BLOCK, rdflag)
	if new_ind {
		zero_block(bp, INDIRECT_BLOCK, fi.blocksize)
	}
	ex = excess
	wr_indir(bp, ex, int(new_zone))
	bp.dirty = true
	fi.cache.PutBlock(bp, INDIRECT_BLOCK)
	return nil
}

// Given a pointer to an indirect block, write one entry
func wr_indir(bp *CacheBlock, index int, zone int) {
	indb := bp.block.(IndirectBlock)
	indb[index] = uint32(zone)
}
