package fs

import (
	"github.com/jnwhiteh/minixfs/common"
	"sync"
)

// A filp is a potentially shared instance of an open file. It implements the
// interface used by programs to perform file input/output. It also serves as
// a layer for error-checking, to ensure correct behaviour when a program
// attempts to use an invalid file descriptor.
//
// This is implemented using a mutex because all operations require exclusive
// access to the resource.
type filp struct {
	count int           // the number of clients
	pos   int           // the current position in the file
	file  common.File   // the file server backing the operations
	inode *common.Inode // the inode this refers to

	mode uint16 // the mode under which this file was opened

	m *sync.Mutex // for mutual exclusion
}

func (fi *filp) Seek(pos, whence int) (int, error) {
	fi.m.Lock()
	defer fi.m.Unlock()

	if fi.file == nil {
		return -1, common.EBADF
	}

	switch whence {
	case 1:
		fi.pos += pos
	case 0:
		fi.pos = pos
	default:
		panic("NYI: Seek with whence > 1")
	}

	return fi.pos, nil
}

func (fi *filp) Read(buf []byte) (int, error) {
	fi.m.Lock()
	defer fi.m.Unlock()

	if fi.file == nil {
		return 0, common.EBADF
	}

	n, err := fi.file.Read(buf, fi.pos)
	fi.pos += n

	return n, err
}

func (fi *filp) Write(buf []byte) (int, error) {
	fi.m.Lock()
	defer fi.m.Unlock()

	if fi.file == nil {
		return 0, common.EBADF
	}

	n, err := fi.file.Write(buf, fi.pos)
	fi.pos += n

	return n, err
}

func (fi *filp) Truncate(length int) error {
	fi.m.Lock()
	defer fi.m.Unlock()

	if fi.file == nil {
		return common.EBADF
	}

	fi.pos = length

	return fi.file.Truncate(length)
}

func (fi *filp) Fstat() (*common.StatInfo, error) {
	fi.m.Lock()
	defer fi.m.Unlock()

	if fi.file == nil {
		return nil, common.EBADF
	}

	return fi.file.Fstat()
}

// This function is not exposed to the user, it only exists to perform the
// cleanup part of the close() system call. Accordingly, it will only be
// acquired when the file system is locked for that call, so it can safely
// shut down the file server.
func (fi *filp) Close() error {
	fi.m.Lock()
	defer fi.m.Unlock()

	fi.count--
	if fi.count == 0 {
		return fi.file.Close()
	}
	return nil
}
