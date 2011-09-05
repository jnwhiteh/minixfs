package minixfs

import (
	"os"
)

// File represents an open file and is the OO equivalent of the file
// descriptor.
type File struct {
	*filp           // the current position in the file
	proc   *Process // the process in which this file is opened
	fd     int      // the numeric file descriptor in the process for this file
	finode *Finode  // the thread-safe interface to the 'finode'
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

	n, err := file.finode.Read(b, file.Pos())
	file.SetPosDelta(n)

	return n, err
}

// Write a slice of bytes to the file at the current position. Returns the
// number of bytes actually written and an error (if any).
func (file *File) Write(data []byte) (n int, err os.Error) {
	if file.fd == NO_FILE {
		return 0, EBADF
	}

	pos := file.Pos()
	// Check for O_APPEND flag
	if file.flags&O_APPEND > 0 {
		fsize := int(file.inode.Size())
		pos = fsize
	}

	n, err = file.finode.Write(data, pos)
	file.SetPos(pos + n)

	return n, err
}

// A non-locking version of the close logic, to be called from proc.Exit and
// file.Close().
func (file *File) close() {
	file.proc.fs.put_inode(file.inode)

	proc := file.proc
	proc.filp[file.fd] = nil
	proc._files[file.fd] = nil

	file.filp.SetCountDelta(-1)
	file.proc = nil
	file.fd = NO_FILE
}
