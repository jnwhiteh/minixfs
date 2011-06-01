package minixfs

import "math"
import "os"
import "sync"

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
	Z_Search uint // when searching for an unused zone, start at this bit

	isup   *Inode // inode for root dir of mounted file system
	imount *Inode // inode mounted on

	m *sync.RWMutex // r/w mutex for working with bitmaps/I_Search
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
func ReadSuperblock(dev BlockDevice) (*Superblock, os.Error) {
	sup_disk := new(disk_superblock)
	err := dev.Read(sup_disk, 1024)
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
		m:                 new(sync.RWMutex),
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
	sup.m = new(sync.RWMutex)
	return sup, nil
}

// Allocate a bit from a bit map and return its bit number
func (fs *FileSystem) alloc_bit(dev int, bmap uint, origin uint) uint {
	var start_block uint // first bit block
	var map_bits uint    // how many bits are there in the bit map
	var bit_blocks uint  // how many blocks are there in the bit map

	super := fs.supers[dev]
	super.m.Lock() // we're altering the bitmaps
	defer super.m.Unlock()

	if bmap == IMAP {
		start_block = START_BLOCK
		map_bits = super.Ninodes + 1
		bit_blocks = super.Imap_blocks
	} else {
		start_block = START_BLOCK + super.Imap_blocks
		map_bits = super.Zones - (super.Firstdatazone_old - 1)
		bit_blocks = super.Zmap_blocks
	}

	// Figure out where to start the bit search (depends on 'origin')
	if origin >= map_bits {
		origin = 0 // for robustness
	}

	// Locate the starting place
	block := origin / _FS_BITS_PER_BLOCK(super.Block_size)
	word := (origin % _FS_BITS_PER_BLOCK(super.Block_size)) / FS_BITCHUNK_BITS

	// Iterate over all blocks plus one, because we start in the middle
	bcount := bit_blocks + 1
	//wlim := FS_BITMAP_CHUNKS(fs.Block_size)

	for {
		bp := fs.get_block(dev, int(start_block+block), MAP_BLOCK)
		bitmaps := bp.block.(MapBlock)

		// Iterate over the words in a block
		for i := word; i < uint(len(bitmaps)); i++ {
			num := bitmaps[i]

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
			b := (block * _FS_BITS_PER_BLOCK(super.Block_size)) + (i * FS_BITCHUNK_BITS) + bit

			// Don't allocate bits beyond the end of the map
			if b >= map_bits {
				break
			}

			// Allocate and return bit number
			num = num | (1 << bit)
			bitmaps[i] = num

			bp.dirty = true
			fs.put_block(bp, MAP_BLOCK)
			return b
		}

		fs.put_block(bp, MAP_BLOCK)
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
func (fs *FileSystem) free_bit(dev int, bmap uint, bit_returned uint) {
	var start_block uint // first bit block

	super := fs.supers[dev]
	super.m.Lock() // we're altering the bitmaps
	defer super.m.Unlock()

	if bmap == IMAP {
		start_block = START_BLOCK
	} else {
		start_block = START_BLOCK + super.Imap_blocks
	}

	block := bit_returned / _FS_BITS_PER_BLOCK(super.Block_size)
	word := (bit_returned % _FS_BITS_PER_BLOCK(super.Block_size)) / FS_BITCHUNK_BITS

	bit := bit_returned % FS_BITCHUNK_BITS
	mask := uint16(1) << bit

	bp := fs.get_block(dev, int(start_block+block), MAP_BLOCK)
	bitmaps := bp.block.(MapBlock)

	k := bitmaps[word]
	if (k & mask) == 0 {
		if bmap == IMAP {
			panic("tried to free unused inode")
		} else if bmap == ZMAP {
			panic("tried to free unused block")
		}
	}

	k = k & (^mask)
	bitmaps[word] = k
	bp.dirty = true
	fs.put_block(bp, MAP_BLOCK)
}

// Return a zone
func (fs *FileSystem) free_zone(dev int, numb uint) {
	super := fs.supers[dev]
	if numb < super.Firstdatazone_old || numb >= super.Nzones {
		return
	}
	bit := numb - super.Firstdatazone_old - 1
	fs.free_bit(dev, ZMAP, bit)

	super.m.Lock() // examining/altering super.Z_Search
	if bit < super.Z_Search {
		super.Z_Search = bit
	}
	super.m.Unlock()
}
