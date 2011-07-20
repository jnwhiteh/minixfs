package minixfs

import (
	"os"
)

var inUse *filp = new(filp)

// Find an available file descriptor slot/filp table entry and reserve them as
// allocated, using the 'inUse' filp defined above. This filp entry cannot and
// should not be used and should be replaced with the true active filp entry
// to complete the allocation.
func (fs *FileSystem) reserve_fd(proc *Process, start int, mode uint16) (int, int, os.Error) {
	fs.m.filp.Lock()
	defer fs.m.filp.Unlock()

	// Find an available file descriptor slot
	var fd int = -1
	for i := 0; i < OPEN_MAX; i++ {
		if proc.filp[i] == nil && proc.filp[i] != inUse {
			fd = i
			break
		}
	}

	if fd < 0 {
		return -1, -1, EMFILE
	}

	var filpi int = -1
	for i := 0; i < NR_FILPS; i++ {
		if proc.fs._filp[i] == nil && proc.fs._filp[i] != inUse {
			filpi = i
			break
		}
	}

	if filpi < 0 {
		return -1, -1, ENFILE
	}

	proc.filp[fd] = inUse
	proc.fs._filp[filpi] = inUse

	return fd, filpi, nil
}
