package minixfs

// Skeleton implementation of system calls required for tests in 'fs_test.go'
type Process struct {
	fs      *fileSystem // the file system on which this process resides
	pid     int         // numeric id of the process
	umask   uint16      // file creation mask
	rootdir *Inode      // root directory of the process
	workdir *Inode      // working directory of the process
	filp    []*filp     // the list of file descriptors
	_files  []*File     // the list of open files
}
