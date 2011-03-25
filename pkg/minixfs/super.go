package minixfs

import "math"
import "os"
import "encoding/binary"

type disk_superblock struct {
	Ninodes           uint32 // # of usable inodes on the minor device
	Nzones            uint16 // total device size, including bit maps, etc.
	Imap_blocks       uint16 // # of blocks used by inode bit map
	Zmap_blocks       uint16 // # of blocks used by zone bit map
	Firstdatazone_old uint16 // number of first data zone
	Log_zone_size     uint16 // log2 of blocks/zone
	Pad               uint16 // try to avoid compiler-dependent padding
	Max_size          int32  // maximum file size on this device
	Zones             uint32 // number of zones (replaces s_nzones in V2+)
	Magic             uint16 // magic number to recognize super-blocks

	// The following fields are only present in V3 and above, which is
	// the standard version of Minix that we are implementing
	Pad2         uint16 // try to avoid compiler-dependent padding
	Block_size   uint16 // block size in bytes
	Disk_version byte   // filesystem format sub-version
}

type Superblock struct {
	*disk_superblock
	inodes_per_block uint
}

func bitmapsize(nr_bits uint, block_size uint) uint {
	// In this assignment, 2 == usizeof(bitchunk_t)
	var bchunks uint = block_size / 2
	var bchunk_bits uint = 2 * CHAR_BIT
	bits_per_block := bchunks * bchunk_bits

	var nr_blocks uint = nr_bits / bits_per_block
	if (nr_blocks * bits_per_block) < nr_bits {
		nr_blocks = nr_blocks + 1
	}
	return nr_blocks
}

// Read the superblock from the second 1024k block of the file
func ReadSuperblock(file *os.File) (*Superblock, os.Error) {
	sup_disk := new(disk_superblock)
	_, err := file.Seek(1024, 0)
	if err != nil {
		return nil, err
	}
	err = binary.Read(file, binary.LittleEndian, sup_disk)
	if err != nil {
		return nil, err
	}

	ipb := sup_disk.Block_size / V2_INODE_SIZE
	sup := &Superblock{sup_disk, uint(ipb)}
	return sup, nil
}

// Creates a new superblock data structure based on specified parameters
func NewSuperblock(blocks, inodes, block_size uint) (*Superblock, os.Error) {
	sup := new(Superblock)

	inodes_per_block := block_size / V2_INODE_SIZE

	// Check to see if inode count is automatic (0) and adjust accordingly
	if inodes == 0 {

		kb := (blocks * block_size) / 1024
		inodes = kb / 2
		if kb >= 100000 {
			inodes = kb / 4
		}

		// round up to fill inode block
		inodes = inodes + inodes_per_block - 1
		inodes = inodes / inodes_per_block * inodes_per_block
	}

	if inodes < 1 {
		return nil, os.NewError("Inode count is too small")
	}

	zones := blocks >> ZONE_SHIFT

	sup.Ninodes = uint32(inodes)
	if uint(sup.Ninodes) != inodes {
		return nil, os.NewError("Inode count is too high, need fewer inodes")
	}

	sup.Nzones = 0
	sup.Zones = uint32(zones)

	// Perform a check here to see if we need a larger block size
	// for a filesystem of the given size. This is accomplished
	// by checking overflow when assigned to the struct
	nb := bitmapsize(1+inodes, block_size)
	sup.Imap_blocks = uint16(nb)
	if uint(sup.Imap_blocks) != nb {
		return nil, os.NewError("Too many inode bitmap blocks, please try a larger block size")
	}

	nb = bitmapsize(zones, block_size)
	sup.Zmap_blocks = uint16(nb)
	if uint(sup.Imap_blocks) != nb {
		return nil, os.NewError("Too many zone bitmap blocks, please try a larger block size")
	}

	inode_offset := START_BLOCK + sup.Imap_blocks + sup.Zmap_blocks
	inodeblks := uint16((inodes + inodes_per_block - 1) / inodes_per_block)
	initblks := inode_offset + inodeblks
	nb = uint((initblks + (1 << ZONE_SHIFT) - 1) >> ZONE_SHIFT)
	if nb >= zones {
		return nil, os.NewError("Bitmaps are too large")
	}
	sup.Firstdatazone_old = uint16(nb)
	if uint(sup.Firstdatazone_old) != nb {
		// The field is too small to store the value. Fortunately, the value
		// can be computed from other fields. We set the on-disk field to zero
		// to indicate that it must not be u sed. Eventually we can always set
		// the on-disk field to zero, and stop using it.
		sup.Firstdatazone_old = 0
	}
	sup.Log_zone_size = ZONE_SHIFT

	v2indirect := (block_size / V2_ZONE_NUM_SIZE)
	v2sq := v2indirect * v2indirect
	zo := V2_NR_DZONES + v2indirect + v2sq

	sup.Magic = SUPER_V3
	sup.Block_size = uint16(block_size)
	if uint(sup.Block_size) != block_size {
		return nil, os.NewError("Block size is too large, please choose a smaller one")
	}
	if math.MaxUint32/block_size < zo {
		sup.Max_size = math.MaxInt32
	} else {
		sup.Max_size = int32(zo * block_size)
		if uint(sup.Max_size) != (zo * block_size) {
			return nil, os.NewError("Maximum file size is too large")
		}
	}

	return sup, nil
}
