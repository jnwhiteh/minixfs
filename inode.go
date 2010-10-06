package main

type inode struct {
	i_mode mode_t		// file type, protection, er.
	i_nlinks nlink_t	// how many links to this file
	i_uid uid_t			// user id of the file's owner
	i_gid gid_t			// group number
	i_size off_t		// current file size in bytes
	i_atime time_t	// time of last access (C2 only)
	i_mtime time_t	// when was file data last changed
	i_ctime time_t	// when was inode itself changed (V2 only)
	i_zone [V2_NR_TZONES]zone_t	// zone numbers for direct, ind, and dbl ind

	// The following items are not present on the disk
	i_dev dev_t	// which device is the inode on
	i_num ino_t	// inode number on its (minor) device
	i_count int	// # times inode used; 0 means slot is free
	i_ndzones int	// # direct zones (Vx_NR_DZONES)
	i_nindirs int	// # indirect zones per indirect block
	i_sp *super_block	// pointer to super block for inode's device
	i_dirt byte	// CLEAN or DIRTY
	i_pipe byte // set to I_PIPE if pipe
	i_mount byte // this bit is set if file mounted on
	i_seek byte	// set on LSEEK, cleared on READ/WRITE
	i_update byte // the ATIME, CTIME, and MTIME bits are here
}
