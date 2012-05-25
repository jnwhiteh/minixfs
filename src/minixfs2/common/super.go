package common

var NO_DEVINFO = DeviceInfo{}

// Read the superblock from the seconds 1024k block of the file and perform
// some calculations to provide the basic device information needed throughout
// the file system.
func GetDeviceInfo(dev BlockDevice) (DeviceInfo, error) {
	sup := new(Disk_Superblock)
	err := dev.Read(sup, 1024)
	if err != nil {
		return NO_DEVINFO, err
	}

	info := DeviceInfo{
		int(sup.Imap_blocks + sup.Zmap_blocks + 2),
		int(sup.Block_size),
		uint(sup.Log_zone_size),
		int(sup.Firstdatazone),
		int(sup.Zones),
		int(sup.Ninodes),
		int(sup.Max_size),
		int(sup.Imap_blocks),
		int(sup.Zmap_blocks),
		NO_DEV,
		nil,
	}

	return info, nil
}

//////////////////////////////////////////////////////////////////////////////
// Utility function for creating a new superblock
//////////////////////////////////////////////////////////////////////////////

// // Creates a new superblock data structure based on specified parameters
// func FormatSuperblock(blocks, inodes, block_size int) (Superblock, os.Error) {
// 	sup := new(superblock)
//
// 	inodes_per_block := block_size / V2_INODE_SIZE
//
// 	// Check to see if inode count is automatic (0) and adjust accordingly
// 	if inodes == 0 {
//
// 		kb := (blocks * block_size) / 1024
// 		inodes = kb / 2
// 		if kb >= 100000 {
// 			inodes = kb / 4
// 		}
//
// 		// round up to fill inode block
// 		inodes = inodes + inodes_per_block - 1
// 		inodes = inodes / inodes_per_block * inodes_per_block
// 	}
//
// 	if inodes < 1 {
// 		return nil, os.NewError("Inode count is too small")
// 	}
//
// 	zones := blocks >> ZONE_SHIFT
//
// 	sup.ninodes = inodes
// 	if sup.ninodes != inodes {
// 		return nil, os.NewError("Inode count is too high, need fewer inodes")
// 	}
//
// 	sup.nzones = 0
// 	sup.zones = zones
//
// 	// Perform a check here to see if we need a larger block size
// 	// for a filesystem of the given size. This is accomplished
// 	// by checking overflow when assigned to the struct
// 	nb := bitmapsize(1+inodes, block_size)
// 	sup.imap_blocks = nb
// 	if sup.imap_blocks != nb {
// 		return nil, os.NewError("Too many inode bitmap blocks, please try a larger block size")
// 	}
//
// 	nb = bitmapsize(zones, block_size)
// 	sup.zmap_blocks = nb
// 	if sup.imap_blocks != nb {
// 		return nil, os.NewError("Too many zone bitmap blocks, please try a larger block size")
// 	}
//
// 	inode_offset := START_BLOCK + sup.imap_blocks + sup.zmap_blocks
// 	inodeblks := (inodes + inodes_per_block - 1) / inodes_per_block
// 	initblks := inode_offset + inodeblks
// 	nb = (initblks + (1 << ZONE_SHIFT) - 1) >> ZONE_SHIFT
// 	if nb >= zones {
// 		return nil, os.NewError("Bitmaps are too large")
// 	}
// 	sup.firstdatazone = nb
// 	if sup.firstdatazone != nb {
// 		// The field is too small to store the value. Fortunately, the value
// 		// can be computed from other fields. We set the on-disk field to zero
// 		// to indicate that it must not be u sed. Eventually we can always set
// 		// the on-disk field to zero, and stop using it.
// 		sup.firstdatazone = 0
// 	}
// 	sup.log_zone_size = ZONE_SHIFT
//
// 	v2indirect := (block_size / V2_ZONE_NUM_SIZE)
// 	v2sq := v2indirect * v2indirect
// 	zo := V2_NR_DZONES + v2indirect + v2sq
//
// 	sup.magic = SUPER_V3
// 	sup.blocksize = block_size
// 	if sup.blocksize != block_size {
// 		return nil, os.NewError("Block size is too large, please choose a smaller one")
// 	}
// 	if math.MaxInt32/block_size < zo {
// 		sup.max_size = math.MaxInt32
// 	} else {
// 		sup.max_size = zo * block_size
// 		if sup.max_size != (zo * block_size) {
// 			return nil, os.NewError("Maximum file size is too large")
// 		}
// 	}
// 	return sup, nil
// }
//
