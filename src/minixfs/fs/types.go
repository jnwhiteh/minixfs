package fs

import (
	. "minixfs/common"
	"sync"
)

type fileSystem interface {
	Shutdown() error
	Mount(dev BlockDevice, path string) error
	Unmount(dev BlockDevice) error
	Spawn(pid int, umask uint16, rootpath string) (*Process, error)
	Exit(proc *Process)
	Open(proc *Process, path string, flags int, mode uint16) (*File, error)
	Close(proc *Process, file *File) error
	Unlink(proc *Process, path string) error
	Mkdir(proc *Process, path string, mode uint16) error
	Rmdir(proc *Process, path string) error
	Chdir(proc *Process, path string) error

	Seek(proc *Process, file *File, pos, whence int) (int, error)
	Read(proc *Process, file *File, b []byte) (int, error)
	Write(proc *Process, file *File, b []byte) (int, error)
}

type mountInfo struct {
	imount InodeId
	isup   InodeId
}

type Filp struct {
	filpidx int // the numeric index of the entry in the global filp table
	mode    uint16
	flags   int
	inode   InodeId
	count   int
	pos     int
}

type Process struct {
	pid     int     // the numeric id of this process
	umask   uint16  // file creation mask
	rootdir InodeId // root directory of the process
	workdir InodeId // working directory of the process
	filp    []*Filp // the list of file descriptors
	files   []*File // the list of open files
	m       *sync.Mutex
}

type File struct {
	*Filp     // the current position in the file, inode, etc.
	fd    int // the numeric file descriptor for this file
}
