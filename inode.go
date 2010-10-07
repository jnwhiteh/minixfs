package main

// mode_t		uint16
// nlink_t		uint16
// uid_t		uint16
// gid_t		uint16
// off_t		uint32
// time_t		uint32
// zone_t		uint32

type inode struct {
	Mode             uint16 // file type, protection, etc.
	Nlinks           uint16 // how many links to this file. HACK!
	Uid              uint16 // user id of the file's owner
	Gid              uint16 // group number. HACK!
	Size             uint32 // current file size in bytes
	Atime            uint32 // when was file data last accessed
	Mtime            uint32 // when was file data last changed
	Ctime            uint32 // when was inode data last changed
	Zone             [10]uint32
}
