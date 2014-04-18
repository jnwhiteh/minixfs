package alloctbl

import "log"
import "math"
import "github.com/jnwhiteh/minixfs/common"

const FS_BITCHUNK_BITS = 16 // the number of bits in a bitchunk_t

type server_AllocTbl struct {
	devinfo *common.DeviceInfo
	cache   common.BlockCache // so we can read/write the allocTbl blocks
	devno   int               // the device number of the device with this allocerblock

	inodes_per_block    int // the number of inodes per block
	bitchunks_per_block int // the number of bitchunks (16-bit seqments) per block

	i_search int // start searching for unallocated inodes here
	z_search int // start searching for unallocated zones here

	in  chan reqAllocTbl
	out chan resAllocTbl
}

func NewAllocTbl(devinfo *common.DeviceInfo, cache common.BlockCache, devno int) common.AllocTbl {
	alloc := &server_AllocTbl{
		devinfo,
		cache,
		devno,
		devinfo.Blocksize / common.V2_INODE_SIZE,
		(devinfo.Blocksize / 2) * 8,
		0,
		0,
		make(chan reqAllocTbl),
		make(chan resAllocTbl),
	}

	go alloc.loop()
	return alloc
}
func (alloc *server_AllocTbl) loop() {
	alive := true
	for alive {
		req := <-alloc.in
		switch req := req.(type) {
		case req_AllocTbl_AllocInode:
			b := alloc.alloc_bit(common.IMAP, alloc.i_search)

			if b == common.NO_BIT {
				log.Printf("Out of i-nodes on device")
				alloc.out <- res_AllocTbl_AllocInode{common.NO_INODE, common.ENFILE}
				continue
			}

			alloc.i_search = b // next time start here
			alloc.out <- res_AllocTbl_AllocInode{b, nil}
		case req_AllocTbl_AllocZone:
			var bstart int

			if req.zstart <= alloc.devinfo.Firstdatazone {
				bstart = alloc.z_search
			} else {
				bstart = req.zstart - (alloc.devinfo.Firstdatazone - 1)
			}

			bit := alloc.alloc_bit(common.ZMAP, bstart)
			if bit == common.NO_BIT {
				if alloc.devno == common.ROOT_DEVICE {
					log.Printf("No space on rootdevice %d", alloc.devno)
				} else {
					log.Printf("No space on device %d", alloc.devno)
				}
				alloc.out <- res_AllocTbl_AllocZone{common.NO_ZONE, common.ENOSPC}
				continue
			}

			if bit < alloc.z_search || alloc.z_search == common.NO_BIT {
				alloc.z_search = bit
			}
			alloc.out <- res_AllocTbl_AllocZone{(alloc.devinfo.Firstdatazone - 1) + bit, nil}
		case req_AllocTbl_FreeInode:
			if req.inum <= 0 || req.inum > alloc.devinfo.Inodes {
				alloc.out <- res_AllocTbl_FreeInode{}
				continue
			}
			alloc.free_bit(common.IMAP, req.inum)
			if req.inum < alloc.i_search {
				alloc.i_search = req.inum
			}
			alloc.out <- res_AllocTbl_FreeInode{}
		case req_AllocTbl_FreeZone:
			if req.znum < alloc.devinfo.Firstdatazone || req.znum >= alloc.devinfo.Zones {
				alloc.out <- res_AllocTbl_FreeZone{}
				continue
			}

			// Turn this from an absolute zone into a bit number
			bit := req.znum - (alloc.devinfo.Firstdatazone - 1)
			alloc.free_bit(common.ZMAP, bit)

			if bit < alloc.z_search || alloc.z_search == common.NO_BIT {
				alloc.z_search = bit
			}
			alloc.out <- res_AllocTbl_FreeZone{}
		case req_AllocTbl_Shutdown:
			// This is always successful
			alive = false
			alloc.out <- res_AllocTbl_Shutdown{nil}
		}
	}
}

// Allocate a bit from a bit map and return its bit number
func (alloc *server_AllocTbl) alloc_bit(which int, origin int) int {
	var start_block int // first bit block
	var map_bits int    // how many bits are there in the bit map
	var bit_blocks int  // how many blocks are there in the bit map

	if which == common.IMAP {
		start_block = common.START_BLOCK
		map_bits = alloc.devinfo.Inodes + 1
		bit_blocks = alloc.devinfo.ImapBlocks
	} else {
		start_block = common.START_BLOCK + alloc.devinfo.ImapBlocks
		map_bits = alloc.devinfo.Zones - (alloc.devinfo.Firstdatazone - 1)
		bit_blocks = alloc.devinfo.ZmapBlocks
	}

	// Figure out where to start the bit search (depends on 'origin')
	if origin >= map_bits {
		origin = 0 // for robustness
	}

	// Locate the starting place
	block := origin / alloc.bitchunks_per_block
	word := (origin % alloc.bitchunks_per_block) / FS_BITCHUNK_BITS

	// Iterate over all blocks plus one, because we start in the middle
	bcount := bit_blocks + 1
	//wlim := FS_BITMAP_CHUNKS(fs.Block_size)

	for {
		bp := alloc.cache.GetBlock(alloc.devno, int(start_block+block), common.MAP_BLOCK, common.NORMAL)
		allocTbls := bp.Block.(common.MapBlock)

		// Iterate over the words in a block
		for i := word; i < len(allocTbls); i++ {
			num := allocTbls[i]

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
			b := (block * alloc.bitchunks_per_block) + (i * FS_BITCHUNK_BITS) + int(bit)

			// Don't allocate bits beyond the end of the map
			if b >= map_bits {
				break
			}

			// Allocate and return bit number
			num = num | (1 << bit)
			allocTbls[i] = num

			bp.Dirty = true
			alloc.cache.PutBlock(bp, common.MAP_BLOCK)
			return b
		}

		alloc.cache.PutBlock(bp, common.MAP_BLOCK)
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

	return common.NO_BIT
}

// Deallocate an inode/zone in the allocTbl, freeing it up for re-use
func (alloc *server_AllocTbl) free_bit(which int, bit_returned int) {
	var start_block int // first bit block

	if which == common.IMAP {
		start_block = common.START_BLOCK
	} else {
		start_block = common.START_BLOCK + alloc.devinfo.ImapBlocks
	}

	block := bit_returned / alloc.bitchunks_per_block
	word := (bit_returned % alloc.bitchunks_per_block) / FS_BITCHUNK_BITS

	bit := bit_returned % FS_BITCHUNK_BITS
	mask := uint16(1) << uint(bit)

	bp := alloc.cache.GetBlock(alloc.devno, int(start_block+block), common.MAP_BLOCK, common.NORMAL)
	allocTbls := bp.Block.(common.MapBlock)

	k := allocTbls[word]
	if (k & mask) == 0 {
		if which == common.IMAP {
			panic("tried to free unused inode")
		} else if which == common.ZMAP {
			panic("tried to free unused block")
		}
	}

	k = k & (^mask)
	allocTbls[word] = k
	bp.Dirty = true
	alloc.cache.PutBlock(bp, common.MAP_BLOCK)
}
