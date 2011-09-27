package super

import "log"
import "math"
import . "../../minixfs/common/_obj/minixfs/common"
import "os"

const FS_BITCHUNK_BITS = 16 // the number of bits in a bitchunk_t

// superblock implements the common.Superblock interface, providing a
// re-entrant implementation of zone/inode allocation.
type superblock struct {
	diskblock *Disk_Superblock // the information stored on the disk

	inodes_per_block int // the number of inodes per block
	bits_per_block   int // the number of bits in a bitmap block
	i_search         int // start searching for unallocated inodes here
	z_search         int // start searchign for unallocated zones here

	isup   *CacheInode // inode for root dir of mounted file system
	imount *CacheInode // the inode on which this filesystem is mounted

	// copies of the data stored in diskblock, normalized to integers
	ninodes       int
	nzones        int
	imap_blocks   int
	zmap_blocks   int
	firstdatazone int
	log_zone_size int
	max_size      int
	zones         int
	magic         int
	blocksize     int
	disk_version  byte

	// channels for requests/responses
	in  chan m_super_req
	out chan m_super_res

	cache BlockCache // so we can read/write the bitmap blocks
	devno int        // the device number of the device with this superblock
}

func NewSuperblock(sup_disk *Disk_Superblock) Superblock {
	sup := &superblock{
		diskblock:        sup_disk,
		inodes_per_block: int(sup_disk.Block_size / V2_INODE_SIZE),
		bits_per_block:   (int(sup_disk.Block_size) / 2) * 8,
		ninodes:          int(sup_disk.Ninodes),
		nzones:           int(sup_disk.Nzones),
		imap_blocks:      int(sup_disk.Imap_blocks),
		zmap_blocks:      int(sup_disk.Zmap_blocks),
		firstdatazone:    int(sup_disk.Firstdatazone),
		log_zone_size:    int(sup_disk.Log_zone_size),
		max_size:         int(sup_disk.Max_size),
		zones:            int(sup_disk.Zones),
		magic:            int(sup_disk.Magic),
		blocksize:        int(sup_disk.Block_size),
		disk_version:     sup_disk.Disk_version,
		in:               make(chan m_super_req),
		out:              make(chan m_super_res),
	}
	go sup.loop()
	return sup
}

func (sup *superblock) loop() {
	var in <-chan m_super_req = sup.in
	var out chan<- m_super_res = sup.out

	for req := range in {
		switch req := req.(type) {
		case m_super_req_alloc_inode:
			inum, err := sup.alloc_inode(req.mode)
			out <- m_super_res_alloc_inode{inum, err}
		case m_super_req_alloc_zone:
			znum, err := sup.alloc_zone(req.zstart)
			out <- m_super_res_alloc_zone{znum, err}
		case m_super_req_free_inode:
			sup.free_inode(req.inum)
			out <- m_super_res_empty{}
		case m_super_req_free_zone:
			sup.free_zone(req.znum)
			out <- m_super_res_empty{}
		case m_super_req_close:
			out <- m_super_res_err{nil}
			close(sup.in)
			close(sup.out)
		}
	}
}

//////////////////////////////////////////////////////////////////////////////
// Interface implementations (re-entrant)
//////////////////////////////////////////////////////////////////////////////

func (sup *superblock) AllocInode(mode uint16) (int, os.Error) {
	sup.in <- m_super_req_alloc_inode{mode}
	res := (<-sup.out).(m_super_res_alloc_inode)
	return res.inum, res.err
}

func (sup *superblock) AllocZone(zone int) (int, os.Error) {
	sup.in <- m_super_req_alloc_zone{zone}
	res := (<-sup.out).(m_super_res_alloc_zone)
	return res.znum, res.err
}

func (sup *superblock) FreeInode(inum int) {
	sup.in <- m_super_req_free_inode{inum}
	<-sup.out // empty response
}

func (sup *superblock) FreeZone(znum int) {
	sup.in <- m_super_req_free_zone{znum}
	<-sup.out // empty response
}

func (sup *superblock) Close() os.Error {
	sup.in <- m_super_req_close{}
	res := (<-sup.out).(m_super_res_err)
	return res.err
}

//////////////////////////////////////////////////////////////////////////////
// Private implememntations
//////////////////////////////////////////////////////////////////////////////

// Allocate a free inode on the given device and return its inum
func (sup *superblock) alloc_inode(mode uint16) (int, os.Error) {
	b := sup.alloc_bit(IMAP, sup.i_search)

	if b == NO_BIT {
		log.Printf("Out of i-nodes on device")
		return NO_INODE, ENFILE
	}

	sup.i_search = b // next time start here
	return b, nil
}

// Return an inode to the pool of free inodes
func (sup *superblock) free_inode(inum int) {
	if inum <= 0 || inum > sup.ninodes {
		return
	}
	sup.free_bit(IMAP, inum)
	if inum < sup.i_search {
		sup.i_search = inum
	}
}

// Allocate a new zone
func (sup *superblock) alloc_zone(zstart int) (int, os.Error) {
	var bstart int

	// If z is 0, skip initial part of the map known to be fully in use
	if zstart == sup.firstdatazone {
		bstart = sup.z_search
	} else {
		bstart = zstart - (sup.firstdatazone - 1)
	}

	bit := sup.alloc_bit(ZMAP, bstart)
	if bit == NO_BIT {
		if sup.devno == ROOT_DEVICE {
			log.Printf("No space on rootdevice %d", sup.devno)
		} else {
			log.Printf("No space on device %d", sup.devno)
		}
		return NO_ZONE, ENOSPC
	}

	if zstart == sup.firstdatazone {
		sup.z_search = bit
	}

	return int(sup.firstdatazone - 1 + bit), nil
}

// Free an allocated zone so it can be re-used
func (sup *superblock) free_zone(znum int) {
	if znum < sup.firstdatazone || znum >= sup.nzones {
		return
	}
	bit := znum - sup.firstdatazone - 1
	sup.free_bit(ZMAP, bit)

	if bit < sup.z_search {
		sup.z_search = bit
	}
}

// Allocate a bit from a bit map and return its bit number
func (sup *superblock) alloc_bit(bmap int, origin int) int {
	var start_block int // first bit block
	var map_bits int    // how many bits are there in the bit map
	var bit_blocks int  // how many blocks are there in the bit map

	if bmap == IMAP {
		start_block = START_BLOCK
		map_bits = sup.ninodes + 1
		bit_blocks = sup.imap_blocks
	} else {
		start_block = START_BLOCK + sup.imap_blocks
		map_bits = sup.zones - (sup.firstdatazone - 1)
		bit_blocks = sup.zmap_blocks
	}

	// Figure out where to start the bit search (depends on 'origin')
	if origin >= map_bits {
		origin = 0 // for robustness
	}

	// Locate the starting place
	block := origin / sup.bits_per_block
	word := (origin % sup.bits_per_block) / FS_BITCHUNK_BITS

	// Iterate over all blocks plus one, because we start in the middle
	bcount := bit_blocks + 1
	//wlim := FS_BITMAP_CHUNKS(fs.Block_size)

	for {
		bp := sup.cache.GetBlock(sup.devno, int(start_block+block), MAP_BLOCK, NORMAL)
		bitmaps := bp.Block.(MapBlock)

		// Iterate over the words in a block
		for i := word; i < len(bitmaps); i++ {
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
			b := (block * sup.bits_per_block) + (i * FS_BITCHUNK_BITS) + int(bit)

			// Don't allocate bits beyond the end of the map
			if b >= map_bits {
				break
			}

			// Allocate and return bit number
			num = num | (1 << bit)
			bitmaps[i] = num

			bp.Dirty = true
			sup.cache.PutBlock(bp, MAP_BLOCK)
			return b
		}

		sup.cache.PutBlock(bp, MAP_BLOCK)
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
func (sup *superblock) free_bit(bmap int, bit_returned int) {
	var start_block int // first bit block

	if bmap == IMAP {
		start_block = START_BLOCK
	} else {
		start_block = START_BLOCK + sup.imap_blocks
	}

	block := bit_returned / sup.bits_per_block
	word := (bit_returned % sup.bits_per_block) / FS_BITCHUNK_BITS

	bit := bit_returned % FS_BITCHUNK_BITS
	mask := uint16(1) << uint(bit)

	bp := sup.cache.GetBlock(sup.devno, int(start_block+block), MAP_BLOCK, NORMAL)
	bitmaps := bp.Block.(MapBlock)

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
	bp.Dirty = true
	sup.cache.PutBlock(bp, MAP_BLOCK)
}

// Deallocate an inode/zone in the bitmap, freeing it up for re-use
func (sup *superblock) check_bit(bmap int, bit_check int) bool {
	var start_block int // first bit block

	if bmap == IMAP {
		start_block = START_BLOCK
	} else {
		start_block = START_BLOCK + sup.imap_blocks
	}

	block := bit_check / sup.bits_per_block
	word := (bit_check % sup.bits_per_block) / FS_BITCHUNK_BITS

	bit := bit_check % FS_BITCHUNK_BITS
	mask := uint16(1) << uint(bit)

	bp := sup.cache.GetBlock(sup.devno, int(start_block+block), MAP_BLOCK, NORMAL)
	bitmaps := bp.Block.(MapBlock)

	k := bitmaps[word]
	sup.cache.PutBlock(bp, MAP_BLOCK)
	return k&mask > 0
}
