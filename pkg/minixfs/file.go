package minixfs

// File represents an open file and is the OO equivalent of the file
// descriptor.
type File struct {
	*filp           // the current position in the file
	proc   *Process // the process in which this file is opened
	fd     int      // the numeric file descriptor in the process for this file
	finode *Finode  // the thread-safe interface to the 'finode'
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
