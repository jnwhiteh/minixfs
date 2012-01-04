package bitmap

import "log"
import "math"
import . "../../minixfs/common/_obj/minixfs/common"
import "os"

const FS_BITCHUNK_BITS = 16 // the number of bits in a bitchunk_t

// bmaperblock implements the common.Superblock interface, providing a
// re-entrant implementation of zone/inode allocation.
type bitmap struct {
	devinfo DeviceInfo
	cache   BlockCache // so we can read/write the bitmap blocks
	devno   int        // the device number of the device with this bmaperblock

	inodes_per_block    int // the number of inodes per block
	bitchunks_per_block int // the number of bitchunks (16-bit seqments) per block

	i_search int // start searching for unallocated inodes here
	z_search int // start searching for unallocated zones here

	// channels for requests/responses
	in  chan m_bitmap_req
	out chan m_bitmap_res
}

func NewBitmap(devinfo DeviceInfo, cache BlockCache, devno int) Bitmap {
	bmap := &bitmap{
		devinfo,
		cache,
		devno,
		devinfo.Blocksize / V2_INODE_SIZE,
		(devinfo.Blocksize / 2) * 8,
		0,
		0,
		make(chan m_bitmap_req),
		make(chan m_bitmap_res),
	}

	go bmap.loop()
	return bmap
}

func (bmap *bitmap) loop() {
	var in <-chan m_bitmap_req = bmap.in
	var out chan<- m_bitmap_res = bmap.out

	for req := range in {
		switch req := req.(type) {
		case m_bitmap_req_alloc_inode:
			inum, err := bmap.alloc_inode()
			out <- m_bitmap_res_alloc_inode{inum, err}
		case m_bitmap_req_alloc_zone:
			znum, err := bmap.alloc_zone(req.zstart)
			out <- m_bitmap_res_alloc_zone{znum, err}
		case m_bitmap_req_free_inode:
			bmap.free_inode(req.inum)
			out <- m_bitmap_res_empty{}
		case m_bitmap_req_free_zone:
			bmap.free_zone(req.znum)
			out <- m_bitmap_res_empty{}
		case m_bitmap_req_close:
			out <- m_bitmap_res_err{nil}
			close(bmap.in)
			close(bmap.out)
		}
	}
}

//////////////////////////////////////////////////////////////////////////////
// Interface implementations (re-entrant)
//////////////////////////////////////////////////////////////////////////////

func (bmap *bitmap) AllocInode() (int, os.Error) {
	bmap.in <- m_bitmap_req_alloc_inode{}
	res := (<-bmap.out).(m_bitmap_res_alloc_inode)
	return res.inum, res.err
}

func (bmap *bitmap) AllocZone(zone int) (int, os.Error) {
	bmap.in <- m_bitmap_req_alloc_zone{zone}
	res := (<-bmap.out).(m_bitmap_res_alloc_zone)
	return res.znum, res.err
}

func (bmap *bitmap) FreeInode(inum int) {
	bmap.in <- m_bitmap_req_free_inode{inum}
	<-bmap.out // empty response
}

func (bmap *bitmap) FreeZone(znum int) {
	bmap.in <- m_bitmap_req_free_zone{znum}
	<-bmap.out // empty response
}

func (bmap *bitmap) Close() os.Error {
	bmap.in <- m_bitmap_req_close{}
	res := (<-bmap.out).(m_bitmap_res_err)
	return res.err
}

//////////////////////////////////////////////////////////////////////////////
// Private implememntations
//////////////////////////////////////////////////////////////////////////////

// Allocate a free inode on the given device and return its inum
func (bmap *bitmap) alloc_inode() (int, os.Error) {
	b := bmap.alloc_bit(IMAP, bmap.i_search)

	if b == NO_BIT {
		log.Printf("Out of i-nodes on device")
		return NO_INODE, ENFILE
	}

	bmap.i_search = b // next time start here
	return b, nil
}

// Return an inode to the pool of free inodes
func (bmap *bitmap) free_inode(inum int) {
	if inum <= 0 || inum > bmap.devinfo.Inodes {
		return
	}
	bmap.free_bit(IMAP, inum)
	if inum < bmap.i_search {
		bmap.i_search = inum
	}
}

// Allocate a new zone. The parameter given is absolute and will be over
// Firstdatazone, or be NO_ZONE, which indicates that there was no idea where
// the search should start.
func (bmap *bitmap) alloc_zone(zstart int) (int, os.Error) {
	var bstart int

	if zstart <= bmap.devinfo.Firstdatazone {
		bstart = bmap.z_search
	} else {
		bstart = zstart - (bmap.devinfo.Firstdatazone - 1)
	}

	bit := bmap.alloc_bit(ZMAP, bstart)
	if bit == NO_BIT {
		if bmap.devno == ROOT_DEVICE {
			log.Printf("No space on rootdevice %d", bmap.devno)
		} else {
			log.Printf("No space on device %d", bmap.devno)
		}
		return NO_ZONE, ENOSPC
	}

	if bit < bmap.z_search || bmap.z_search == NO_BIT {
		bmap.z_search = bit
	}
	return (bmap.devinfo.Firstdatazone - 1) + bit, nil
}

// Free an allocated zone so it can be re-used
func (bmap *bitmap) free_zone(znum int) {
	if znum < bmap.devinfo.Firstdatazone || znum >= bmap.devinfo.Zones {
		return
	}

	// Turn this from an absolute zone into a bit number
	bit := znum - (bmap.devinfo.Firstdatazone - 1)
	bmap.free_bit(ZMAP, bit)

	if bit < bmap.z_search || bmap.z_search == NO_BIT {
		bmap.z_search = bit
	}
}

// Allocate a bit from a bit map and return its bit number
func (bmap *bitmap) alloc_bit(which int, origin int) int {
	var start_block int // first bit block
	var map_bits int    // how many bits are there in the bit map
	var bit_blocks int  // how many blocks are there in the bit map

	if which == IMAP {
		start_block = START_BLOCK
		map_bits = bmap.devinfo.Inodes + 1
		bit_blocks = bmap.devinfo.ImapBlocks
	} else {
		start_block = START_BLOCK + bmap.devinfo.ImapBlocks
		map_bits = bmap.devinfo.Zones - (bmap.devinfo.Firstdatazone - 1)
		bit_blocks = bmap.devinfo.ZmapBlocks
	}

	// Figure out where to start the bit search (depends on 'origin')
	if origin >= map_bits {
		origin = 0 // for robustness
	}

	// Locate the starting place
	block := origin / bmap.bitchunks_per_block
	word := (origin % bmap.bitchunks_per_block) / FS_BITCHUNK_BITS

	// Iterate over all blocks plus one, because we start in the middle
	bcount := bit_blocks + 1
	//wlim := FS_BITMAP_CHUNKS(fs.Block_size)

	for {
		bp := bmap.cache.GetBlock(bmap.devno, int(start_block+block), MAP_BLOCK, NORMAL)
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
			b := (block * bmap.bitchunks_per_block) + (i * FS_BITCHUNK_BITS) + int(bit)

			// Don't allocate bits beyond the end of the map
			if b >= map_bits {
				break
			}

			// Allocate and return bit number
			num = num | (1 << bit)
			bitmaps[i] = num

			bp.Dirty = true
			bmap.cache.PutBlock(bp, MAP_BLOCK)
			return b
		}

		bmap.cache.PutBlock(bp, MAP_BLOCK)
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
func (bmap *bitmap) free_bit(which int, bit_returned int) {
	var start_block int // first bit block

	if which == IMAP {
		start_block = START_BLOCK
	} else {
		start_block = START_BLOCK + bmap.devinfo.ImapBlocks
	}

	block := bit_returned / bmap.bitchunks_per_block
	word := (bit_returned % bmap.bitchunks_per_block) / FS_BITCHUNK_BITS

	bit := bit_returned % FS_BITCHUNK_BITS
	mask := uint16(1) << uint(bit)

	bp := bmap.cache.GetBlock(bmap.devno, int(start_block+block), MAP_BLOCK, NORMAL)
	bitmaps := bp.Block.(MapBlock)

	k := bitmaps[word]
	if (k & mask) == 0 {
		if which == IMAP {
			panic("tried to free unused inode")
		} else if which == ZMAP {
			panic("tried to free unused block")
		}
	}

	k = k & (^mask)
	bitmaps[word] = k
	bp.Dirty = true
	bmap.cache.PutBlock(bp, MAP_BLOCK)
}

// Check whether or not a given bit in the bitmap is set
func (bmap *bitmap) checkBit(which int, bit_check int) bool {
	var start_block int // first bit block

	if which == IMAP {
		start_block = START_BLOCK
	} else {
		start_block = START_BLOCK + bmap.devinfo.ImapBlocks
	}

	block := bit_check / bmap.bitchunks_per_block
	word := (bit_check % bmap.bitchunks_per_block) / FS_BITCHUNK_BITS

	bit := bit_check % FS_BITCHUNK_BITS
	mask := uint16(1) << uint(bit)

	bp := bmap.cache.GetBlock(bmap.devno, int(start_block+block), MAP_BLOCK, NORMAL)
	bitmaps := bp.Block.(MapBlock)

	k := bitmaps[word]
	bmap.cache.PutBlock(bp, MAP_BLOCK)
	return k&mask > 0
}

var _ Bitmap = &bitmap{}
