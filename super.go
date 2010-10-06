package main

type bit_t uint32
type ino_t ulong
type zone1_t uint16
type dev_t short

type super_block struct {
	s_ninodes ino_t			// # of usable inodes on the minor device
	s_nzones zone1_t		// total device size, including bit maps, etc.
	s_imap_blocks short		// # of blocks used by inode bit map
	s_zmap_blocks short		// # of blocks used by zone bit map
	s_firstdatazone zone1_t	// number of first data zone
	s_log_zone_size short	// log2 of blocks/zone
	s_pad short				// try to avoid compiler-dependent padding
	s_max_size off_t		// maximum file size on this device
	s_zones zone_t			// number of zones (replaces s_nzones in V2)
	s_magic short			// magic number to recognize super-blocks

	// The following items are valid on disk only for V3 and above

	// The block size in bytes. Minimum MIN_BLOCK_SIZE, SECTOR_SIZE
	// multiple. If V1 or V2 filesystem, this should be initialised
	// to STATIC_BLOCK_SIZE. Maximum MAX_BLOCK_SIZE

	s_pad2 short			// try to avoid compiler-dependent padding
	s_block_size ushort		// block size in bytes
	s_disk_version byte		// filesystem format sub-version

	// The following items are only used when the super_block is in memory
	s_isup *inode			// inode for root dir of mounted file system
	s_imount *inode			// inode mounted on
	s_inodes_per_block uint32	// precalculated from magic number
	s_dev dev_t				// whose super block is this?
	s_rd_only int			// set to 1 iff file sys mounted read only
	s_native int			// set to 1 iff not byte swapped file system
	s_version int			// file system version, zero means bad magic
	s_ndzones int			// # of direct zones in an inode
	s_nindirs int			// # of indirect zones per indirect block
	s_isearch bit_t			// inodes below this bit number are in use
	s_zsearch bit_t			// all zones below this bit number are in use
}

