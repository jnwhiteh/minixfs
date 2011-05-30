package minixfs

type filp struct {
	mode uint16
	flags int
	count int
	inode *Inode
	pos int
}
