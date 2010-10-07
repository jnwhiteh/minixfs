package main

type inode struct {				// V2.x disk inode
	Mode uint16					// file type, protection, etc.
	Nlinks uint16				// how many links to this file. HACK!
	Uid uint16 // user id of the file's owner
	Gid uint16					// group number. HACK!
	Size uint32					// current file size in bytes
	Atime uint32					// when was file data last accessed
	Mtime uint32					// when was file data last changed
	Ctime uint32					// when was inode data last changed
	Zone [7]uint32
	Indirect_zone uint32
	DblIndirect_zone uint32
	Unused uint32
}
