package inode

import (
	"io"
	"log"
	. "minixfs/common"
	"minixfs/utils"
)

func Read(rip *Inode, b []byte, pos int) (int, error) {
	// We want to read at most len(b) bytes from the given riple. This data
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

	blocksize := rip.Devinfo.Blocksize

	// We can't just start reading at the start of a block, since we may be at
	// an offset within that block. So work out the riprst chunk to read
	offset := curpos % blocksize
	bnum := ReadMap(rip, curpos, rip.Bcache)

	// TODO: Error check this
	// read the riprst data block and copy the portion of data we need
	bp := rip.Bcache.GetBlock(rip.Devnum, bnum, FULL_DATA_BLOCK, NORMAL)
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

	// we need this entire riprst block, so start riplling
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
		bp := rip.Bcache.GetBlock(rip.Devnum, bnum, FULL_DATA_BLOCK, NORMAL)
		if _, sok := bp.Block.(FullDataBlock); !sok {
			log.Printf("block num: %d", bp.Blockno)
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

// Write len(b) bytes to the riple at position 'pos'
func Write(rip *Inode, data []byte, pos int) (n int, err error) {
	// TODO: This implementation is direct and doesn't match the abstractions
	// in the original source. At some point it should be reviewed.
	cum_io := 0
	position := pos
	fsize := int(rip.Size)

	// Check in advance to see if riple will grow too big
	if position > rip.Devinfo.Maxsize-len(data) {
		return 0, EFBIG
	}

	// Clear the zone containing the current present EOF if hole about to be
	// created. This is necessary because all unwritten blocks prior to the
	// EOF must read as zeros.
	if position > fsize {
		utils.ClearZone(rip, fsize, 0, rip.Bcache)
	}

	bsize := rip.Devinfo.Blocksize
	nbytes := len(data)
	// Split the transfer into chunks that don't span two blocks.
	for nbytes != 0 {
		off := (position % bsize)
		var min int
		if nbytes < bsize-off {
			min = nbytes
		} else {
			min = bsize - off
		}
		chunk := min
		if chunk < 0 {
			chunk = bsize - off
		}

		// Read or write 'chunk' bytes, fetch the riprst block
		err = utils.WriteChunk(rip, position, off, chunk, data, rip.Bcache)
		if err != nil {
			break // EOF reached
		}

		// Update counters and pointers
		data = data[chunk:] // user buffer
		nbytes -= chunk     // bytes yet to be written
		cum_io += chunk     // bytes written so far
		position += chunk   // position within the riple
	}

	itype := rip.Mode & I_TYPE
	if itype == I_REGULAR || itype == I_DIRECTORY {
		if position > fsize {
			rip.Size = int32(position)
		}
	}

	// TODO: Update times
	if err == nil {
		rip.Dirty = true
	}

	return cum_io, err

}


