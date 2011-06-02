package minixfs

import (
	"os"
)

func (fs *FileSystem) get_fd(proc *Process, start int, mode uint16) (int, int, *filp, os.Error) {
	// Find a slot in the global filp table
	proc.fs.m.filp.Lock()
	defer proc.fs.m.filp.Unlock()

	// Find an available file descriptor slot
	var fd int = -1
	for i := 0; i < OPEN_MAX; i++ {
		if proc.filp[i] == nil {
			fd = i
			break
		}
	}

	if fd < 0 {
		return -1, -1, nil, EMFILE
	}

	var filpi int = -1
	for i := 0; i < NR_FILPS; i++ {
		if proc.fs.filp[i] == nil {
			filpi = i
			break
		}
	}

	if filpi < 0 {
		return -1, -1, nil, ENFILE
	}

	filp := new(filp)
	proc.filp[fd] = filp
	proc.fs.filp[filpi] = filp

	return fd, filpi, filp, nil
}
