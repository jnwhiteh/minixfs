package minixfs

import (
	"log"
	"os"
)

// File represents an open file and is the OO equivalent of the file
// descriptor.
type File struct {
	*filp          // the current position in the file
	proc  *Process // the process in which this file is opened
	fd    int      // the numeric file descriptor in the process for this file
}

// Seek sets the position for the next read or write to pos, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end of the
// file. It returns the new offset and an Error, if any.
//
// TODO: Implement end of file seek and error checking

func (file *File) Seek(pos int, whence int) (int, os.Error) {
	if file.fd == NO_FILE {
		return 0, EBADF
	}

	file.proc.fs.m.device.RLock()
	defer file.proc.fs.m.device.RUnlock()

	switch whence {
	case 1:
		file.SetPosDelta(pos)
	case 0:
		file.SetPos(pos)
	default:
		panic("NYI: file.Seek with whence > 1")
	}

	return file.Pos(), nil
}

// Read up to len(b) bytes from 'file' from the current position within the
// file.
func (file *File) Read(b []byte) (int, os.Error) {
	if file.fd == NO_FILE {
		return 0, EBADF
	}

	file.proc.fs.m.device.RLock()
	defer file.proc.fs.m.device.RUnlock()

	// We want to read at most len(b) bytes from the given file. This data
	// will almost certainly be split up amongst multiple blocks.
	curpos := file.Pos()

	// Determine what the ending position to be read is
	endpos := curpos + len(b)
	fsize := int(file.inode.Size())
	if endpos >= int(fsize) {
		endpos = int(fsize) - 1
	}

	fs := file.proc.fs
	dev := file.inode.dev
	blocksize := int(fs.supers[dev].Block_size)

	// We can't just start reading at the start of a block, since we may be at
	// an offset within that block. So work out the first chunk to read
	offset := curpos % blocksize
	bnum := fs.read_map(file.inode, uint(curpos))

	// TODO: Error check this
	// read the first data block and copy the portion of data we need
	bp := fs.get_block(dev, int(bnum), FULL_DATA_BLOCK, NORMAL)
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
		fs.put_block(bp, FULL_DATA_BLOCK)
		return len(b), nil
	}

	// we need this entire first block, so start filling
	var numBytes int = 0
	for i := 0; i < blocksize-offset; i++ {
		b[i] = bdata[offset+i]
		numBytes++
	}

	fs.put_block(bp, FULL_DATA_BLOCK)
	curpos += numBytes

	// At this stage, all reads should be on block boundaries. The final block
	// will likely be a partial block, so handle that specially.
	for numBytes < len(b) {
		bnum = fs.read_map(file.inode, uint(curpos))
		bp := fs.get_block(dev, int(bnum), FULL_DATA_BLOCK, NORMAL)
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
			fs.put_block(bp, FULL_DATA_BLOCK)
			return numBytes, nil
		}

		// We need this whole block
		for i := 0; i < len(bdata); i++ {
			b[numBytes] = bdata[i]
			numBytes++
		}

		curpos += len(bdata)
		fs.put_block(bp, FULL_DATA_BLOCK)
	}

	// TODO: Update this as we read block after block?
	file.SetPos(curpos)

	return numBytes, nil
}

// Write a slice of bytes to the file at the current position. Returns the
// number of bytes actually written and an error (if any).
func (file *File) Write(data []byte) (n int, err os.Error) {
	if file.fd == NO_FILE {
		return 0, EBADF
	}

	file.proc.fs.m.device.RLock()
	defer file.proc.fs.m.device.RUnlock()

	// TODO: This implementation is direct and doesn't match the abstractions
	// in the original source. At some point it should be reviewed.
	cum_io := 0
	position := int(file.Pos())
	fsize := int(file.inode.Size())

	fs := file.proc.fs
	super := fs.supers[file.inode.dev]
	// Check in advance to see if file will grow too big
	if position > (int(super.Max_size) - len(data)) {
		return 0, EFBIG
	}

	// Check for O_APPEND flag
	if file.flags&O_APPEND > 0 {
		position = fsize
	}

	// Clear the zone containing the current present EOF if hole about to be
	// created. This is necessary because all unwritten blocks prior to the
	// EOF must read as zeros.
	if position > fsize {
		fs.clear_zone(file.inode, uint(fsize), 0)
	}

	bsize := int(super.Block_size)
	nbytes := len(data)
	// Split the transfer into chunks that don't span two blocks.
	for nbytes != 0 {
		off := (position % bsize)
		chunk := _MIN(nbytes, bsize-off)
		if chunk < 0 {
			chunk = bsize - off
		}

		// Read or write 'chunk' bytes, fetch the first block
		err = fs.write_chunk(file.inode, position, off, chunk, data)
		if err != nil {
			break // EOF reached
		}

		// Update counters and pointers
		data = data[chunk:] // user buffer
		nbytes -= chunk     // bytes yet to be written
		cum_io += chunk     // bytes written so far
		position += chunk   // position within the file
	}

	if file.inode.GetType() == I_REGULAR || file.inode.GetType() == I_DIRECTORY {
		if position > fsize {
			file.inode.SetSize(int32(position))
		}
	}

	file.SetPos(position)

	// TODO: Update times
	if err == nil {
		file.inode.SetDirty(true)
	}

	return cum_io, err
}

// A non-locking version of the close logic, to be called from proc.Exit and
// file.Close().
func (file *File) close() {
	file.proc.fs.put_inode(file.inode)

	proc := file.proc
	proc._filp[file.fd] = nil
	proc._files[file.fd] = nil

	file.filp.SetCountDelta(-1)
	file.proc = nil
	file.fd = NO_FILE
}

// TODO: Should this always be succesful?
func (file *File) Close() os.Error {
	if file.fd == NO_FILE {
		return EBADF
	}

	file.proc.fs.m.device.RLock()
	defer file.proc.fs.m.device.RUnlock()

	file.proc.m_filp.Lock()
	defer file.proc.m_filp.Unlock()

	file.close()
	return nil
}
