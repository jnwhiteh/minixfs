package minixfs

import "log"
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
	diskblock        *disk_superblock
	inodes_per_block uint

	// The following are all copies of the data stored in the disk_superblock
	// but normnalised to use uint/int directly rather than use the sized
	// versions. This is done to simplify the code and remove the need for
	// excessive casting when making calculations.
	Ninodes           uint // # of usable inodes on the minor device
	Nzones            uint // total device size, including bit maps, etc.
	Imap_blocks       uint // # of blocks used by inode bit map
	Zmap_blocks       uint // # of blocks used by zone bit map
	Firstdatazone_old uint // number of first data zone
	Log_zone_size     uint // log2 of blocks/zone
	Pad               uint // try to avoid compiler-dependent padding
	Max_size          uint // maximum file size on this device
	Zones             uint // number of zones (replaces s_nzones in V2+)
	Magic             uint // magic number to recognize super-blocks

	Block_size   uint // block size in bytes
	Disk_version byte // filesystem format sub-version

	I_Search uint // when searching for an unused inode, start at this bit
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
	sup := &Superblock{
		diskblock:         sup_disk,
		inodes_per_block:  uint(ipb),
		Ninodes:           uint(sup_disk.Ninodes),
		Nzones:            uint(sup_disk.Nzones),
		Imap_blocks:       uint(sup_disk.Imap_blocks),
		Zmap_blocks:       uint(sup_disk.Zmap_blocks),
		Firstdatazone_old: uint(sup_disk.Firstdatazone_old),
		Log_zone_size:     uint(sup_disk.Log_zone_size),
		Pad:               uint(sup_disk.Pad),
		Max_size:          uint(sup_disk.Max_size),
		Zones:             uint(sup_disk.Zones),
		Magic:             uint(sup_disk.Magic),
		Block_size:        uint(sup_disk.Block_size),
		Disk_version:      sup_disk.Disk_version,
	}
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

	sup.Ninodes = uint(inodes)
	if uint(sup.Ninodes) != inodes {
		return nil, os.NewError("Inode count is too high, need fewer inodes")
	}

	sup.Nzones = 0
	sup.Zones = uint(zones)

	// Perform a check here to see if we need a larger block size
	// for a filesystem of the given size. This is accomplished
	// by checking overflow when assigned to the struct
	nb := bitmapsize(1+inodes, block_size)
	sup.Imap_blocks = uint(nb)
	if uint(sup.Imap_blocks) != nb {
		return nil, os.NewError("Too many inode bitmap blocks, please try a larger block size")
	}

	nb = bitmapsize(zones, block_size)
	sup.Zmap_blocks = uint(nb)
	if uint(sup.Imap_blocks) != nb {
		return nil, os.NewError("Too many zone bitmap blocks, please try a larger block size")
	}

	inode_offset := START_BLOCK + sup.Imap_blocks + sup.Zmap_blocks
	inodeblks := uint((inodes + inodes_per_block - 1) / inodes_per_block)
	initblks := inode_offset + inodeblks
	nb = uint((initblks + (1 << ZONE_SHIFT) - 1) >> ZONE_SHIFT)
	if nb >= zones {
		return nil, os.NewError("Bitmaps are too large")
	}
	sup.Firstdatazone_old = uint(nb)
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
	sup.Block_size = uint(block_size)
	if uint(sup.Block_size) != block_size {
		return nil, os.NewError("Block size is too large, please choose a smaller one")
	}
	if math.MaxUint32/block_size < zo {
		sup.Max_size = math.MaxInt32
	} else {
		sup.Max_size = uint(zo * block_size)
		if uint(sup.Max_size) != (zo * block_size) {
			return nil, os.NewError("Maximum file size is too large")
		}
	}

	return sup, nil
}

// Allocate a bit from a bit map and return its bit number
func (fs *FileSystem) AllocBit(bmap uint, origin uint) (uint) {
	var start_block uint // first bit block
	var map_bits uint    // how many bits are there in the bit map
	var bit_blocks uint // how many blocks are there in the bit map

	if bmap == IMAP {
		start_block = START_BLOCK
		map_bits = fs.super.Ninodes + 1
		bit_blocks = fs.super.Imap_blocks
	} else {
		start_block = START_BLOCK + fs.super.Imap_blocks
		map_bits = fs.super.Zones - (fs.super.Firstdatazone_old-1)
		bit_blocks = fs.super.Zmap_blocks
	}

	// Figure out where to start the bit search (depends on 'origin')
	if origin >= map_bits {
		origin = 0 // for robustness
	}

	// Locate the starting place
	block := origin / FS_BITS_PER_BLOCK(fs.Block_size)
	word := (origin % FS_BITS_PER_BLOCK(fs.Block_size)) / FS_BITCHUNK_BITS

	// Iterate over all blocks plus one, because we start in the middle
	bcount := bit_blocks + 1
	//wlim := FS_BITMAP_CHUNKS(fs.Block_size)

	for {
		bp, err := fs.GetMapBlock(start_block + block)
		if err != nil {
			log.Printf("Unable to fetch bitmap block %d - %s", block, err)
			return NO_BIT
		}

		// Iterate over the words in a block
		for i := word; i < uint(len(bp.Data)); i++ {
			num := bp.Data[i]

			// Does this word contain a free bit?
			if num == math.MaxUint16 {
				// No bits free, move to next word
				continue
			}

			// Find and allocate the free bit
			var bit uint
			for bit = 0; (num & (1 << bit)) != 0; bit++ {
			}

			// Get the bit number from the start of the bit map
			b := (block * FS_BITS_PER_BLOCK(fs.Block_size)) + (i * FS_BITCHUNK_BITS) + bit

			// Don't allocate bits beyond the end of the map
			if b >= map_bits {
				break
			}

			// Allocate and return bit number
			num = num | (1 << bit)
			bp.Data[i] = num

			bp.buf.dirty = true
			fs.PutBlock(bp, MAP_BLOCK)
			return b
		}

		fs.PutBlock(bp, MAP_BLOCK)
		block = block + 1
		if (block) >= bit_blocks {
			block = 0
		}
		word = 0
		bcount = bcount - 1
		if bcount <= 0 {
			break
		}
	}

	return NO_BIT
}

// Deallocate an inode/zone in the bitmap, freeing it up for re-use
func (fs *FileSystem) FreeBit(bmap uint, bit_returned uint) {
	var start_block uint // first bit block

	if bmap == IMAP {
		start_block = START_BLOCK
	} else {
		start_block = START_BLOCK + fs.super.Imap_blocks
	}

	block := bit_returned / FS_BITS_PER_BLOCK(fs.Block_size)
	word := (bit_returned % FS_BITS_PER_BLOCK(fs.Block_size)) / FS_BITCHUNK_BITS

	bit := bit_returned % FS_BITCHUNK_BITS
	mask := uint16(1) << bit

	bp, err := fs.GetMapBlock(start_block + block)
	if err != nil {
		log.Printf("Unable to fetch bitmap block %d - %s", block, err)
		return
	}

	k := bp.Data[word]
	if (k & mask) == 0 {
		if bmap == IMAP {
			panic("tried to free unused inode")
		} else if bmap == ZMAP {
			panic("tried to free unused block")
		}
	}

	k = k & (^ mask)
	bp.Data[word] = k
	bp.buf.dirty = true
	fs.PutBlock(bp, MAP_BLOCK)
}
