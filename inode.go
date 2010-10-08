package main

// mode_t		uint16
// nlink_t		int16
// uid_t		int16
// gid_t		char
// off_t		int32
// time_t		int32
// zone_t		uint32

type inode struct {
	Mode   uint16 // file type, protection, etc.
	Nlinks int16  // how many links to this file. HACK!
	Uid    int16  // user id of the file's owner
	Gid    byte   // group number. HACK!
	Size   int32  // current file size in bytes
	Atime  int32  // when was file data last accessed
	Mtime  int32  // when was file data last changed
	Ctime  int32  // when was inode data last changed
	Zone   [10]uint32
}
