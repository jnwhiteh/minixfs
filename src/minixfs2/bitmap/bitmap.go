package bitmap

import "fmt"
import "log"
import . "minixfs2/common"
import "sync"

const FS_BITCHUNK_BITS = 16 // the number of bits in a bitchunk_t

type bitmap struct {
	devinfo *DeviceInfo
	cache   BlockCache // so we can read/write the allocTbl blocks
	devno   int        // the device number of the device with this allocerblock

	inodes_per_block    int // the number of inodes per block
	bitchunks_per_block int // the number of bitchunks (16-bit seqments) per block

	i_search int // start searching for unallocated inodes here
	z_search int // start searching for unallocated zones here

	imap []bool
	zmap []bool

	lock *sync.Mutex
}

func NewBitmap(devinfo *DeviceInfo, cache BlockCache, devno int) AllocTbl {
	alloc := &bitmap{
		devinfo,
		cache,
		devno,
		devinfo.Blocksize / V2_INODE_SIZE,
		(devinfo.Blocksize / 2) * 8,
		0,
		0,
		nil,
		nil,
		new(sync.Mutex),
	}

	alloc.imap = alloc.load_map(IMAP)
	alloc.zmap = alloc.load_map(ZMAP)

	return alloc
}

// Allocate a bit from a bit map and return its bit number
func (bmap *bitmap) alloc_bit(which int, origin int) int {
	var limit int   // max bit to search through
	var list []bool // the bitmap to search

	if which == IMAP {
		limit = bmap.devinfo.Inodes + 1
		list = bmap.imap
	} else {
		limit = bmap.devinfo.Zones - (bmap.devinfo.Firstdatazone - 1)
		list = bmap.zmap
	}

	for i := origin; i < limit; i++ {
		if !list[i] {
			list[i] = true
			return i
		}
	}

	return NO_BIT
}

// Deallocate an inode/zone in the allocTbl, freeing it up for re-use
func (bmap *bitmap) free_bit(which int, bit_returned int) {
	var limit int   // max bit to search through
	var list []bool // the bitmap to search

	if which == IMAP {
		limit = bmap.devinfo.Inodes + 1
		list = bmap.imap
	} else {
		limit = bmap.devinfo.Zones - (bmap.devinfo.Firstdatazone - 1)
		list = bmap.zmap
	}

	if bit_returned >= 0 && bit_returned < limit {
		if !list[bit_returned] {
			panic(fmt.Sprintf("Attempt to free an unused bit(%d, %d)", which, bit_returned))
		}

		list[bit_returned] = false
	}
}

func (bmap *bitmap) AllocInode() (int, error) {
	bmap.lock.Lock()
	defer bmap.lock.Unlock()

	b := bmap.alloc_bit(IMAP, bmap.i_search)

	if b == NO_BIT {
		log.Printf("Out of i-nodes on device")
		return NO_INODE, ENFILE
	}

	bmap.i_search = b // next time start here
	return b, nil
}

func (bmap *bitmap) AllocZone(zstart int) (int, error) {
	bmap.lock.Lock()
	defer bmap.lock.Unlock()

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
	return bmap.devinfo.Firstdatazone - 1 + bit, nil
}

func (bmap *bitmap) FreeInode(inum int) error {
	bmap.lock.Lock()
	defer bmap.lock.Unlock()

	if inum <= 0 || inum > bmap.devinfo.Inodes {
		return nil
	}

	bmap.free_bit(IMAP, inum)
	if inum < bmap.i_search {
		bmap.i_search = inum
	}
	return nil
}

func (bmap *bitmap) FreeZone(znum int) error {
	bmap.lock.Lock()
	defer bmap.lock.Unlock()

	if znum < bmap.devinfo.Firstdatazone || znum >= bmap.devinfo.Zones {
		return nil
	}

	// Turn this from an absolute zone into a bit number
	bit := znum - (bmap.devinfo.Firstdatazone - 1)
	bmap.free_bit(ZMAP, bit)

	if bit < bmap.z_search || bmap.z_search == NO_BIT {
		bmap.z_search = bit
	}
	return nil
}

func (bmap *bitmap) Shutdown() error {
	bmap.lock.Lock()
	defer bmap.lock.Unlock()

	return nil
}
