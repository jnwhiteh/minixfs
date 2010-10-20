package minixfs

// This type encapsulates a minix file system, including the shared data
// structures associated with the file system. It abstracts away from the
// file system residing on disk

type FileSystem struct {
	super Superblock // the superblock for the associated file system
	inodes []Inode // a slice containing the inodes for the open files
}

// Create a new FileSystem from a given file on the filesystem
func NewFile(filename string) (*FileSystem) {
	var fs *FileSystem = new(FileSystem)
	return fs
}
