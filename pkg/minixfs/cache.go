package minixfs

// The buffer cache in the Minix file system is built on top of the raw device
// and provides access to recently requested blocks served from memory.
//
// When a block is obtained via 'GetBlock' its count field is incremented and
// marks it in-use. When a block is returned using 'PutBlock', this same field
// is decremented. Only if this field is 0 is the block actually moved to the
// list of blocks available for eviction.

import "fmt"
import "log"
import "os"
import "sync"

type BlockType int

const (
	INODE_BLOCK        BlockType = 0 // inode block
	DIRECTORY_BLOCK    BlockType = 1 // directory block
	INDIRECT_BLOCK     BlockType = 2 // pointer block
	MAP_BLOCK          BlockType = 3 // bit map
	FULL_DATA_BLOCK    BlockType = 5 // data, fully used
	PARTIAL_DATA_BLOCK BlockType = 6 // data, partly used
)

// Buf is a generic 'buffer cache struct that is used throughout the file
// server. We need some metadata in order to make things nicer, so we include
// the block, the block number and the other metadata we need. These will be
// needed for any BlockCache implementation.
type buf struct {
	block   Block // the block data structure
	blocknr int   // the number of this block
	dev     int   // the device number of this block
	dirty   bool  // whether or not the block is dirty (needs to be written)
	count   int   // the number of users of this block

	next *buf // used to link all free bufs in a chain
	prev *buf // used to link all free bufs the other way

	b_hash *buf // used to link all bufs for a hash mask together
}

// LRUCache is a block cache implementation that will evict the least recently
// used block when necessary. The number of cached blocks is static.
type LRUCache struct {
	// These struct elements are duplicates of those that can be found in
	// the FileSystem struct. By duplicating them, we make LRUCache a
	// self-contained data structure that has a well-defined interface.
	devs   []BlockDevice // the block devices that comprise the file system
	supers []*Superblock // the superblocks for the given devices

	buf      []*buf // static list of cache blocks
	buf_hash []*buf // the buffer hash table

	front *buf // a pointer to the least recently used block
	rear  *buf // a pointer to the most recently used block

	m *sync.RWMutex
}

// NewLRUCache creates a new LRUCache with the given size
func NewLRUCache() *LRUCache {
	cache := &LRUCache{
		devs:     make([]BlockDevice, NR_SUPERS),
		supers:   make([]*Superblock, NR_SUPERS),
		buf:      make([]*buf, NR_BUFS),
		buf_hash: make([]*buf, NR_BUF_HASH),
		m:        new(sync.RWMutex),
	}

	// Create all of the entries in buf ahead of time
	for i := 0; i < NR_BUFS; i++ {
		cache.buf[i] = new(buf)
	}

	for i := 1; i < NR_BUFS-1; i++ {
		cache.buf[i].prev = cache.buf[i-1]
		cache.buf[i].next = cache.buf[i+1]
	}

	cache.front = cache.buf[0]
	cache.front.next = cache.buf[1]

	cache.rear = cache.buf[NR_BUFS-1]
	cache.rear.prev = cache.buf[NR_BUFS-2]

	for i := 0; i < NR_BUFS-1; i++ {
		cache.buf[i].b_hash = cache.buf[i].next
	}

	cache.buf_hash[0] = cache.front
	return cache
}

// Associate a BlockDevice and *Superblock with a device number so it can be
// used internally. This operation requires the write portion of the RWMutex
// since it alters the devs and supers arrays.
func (c *LRUCache) MountDevice(devno int, dev BlockDevice, super *Superblock) os.Error {
	c.m.Lock()         // acquire the write mutex (+++)
	defer c.m.Unlock() // defer release of the write mutex (---)
	if c.devs[devno] != nil || c.supers[devno] != nil {
		return EBUSY
	}
	c.devs[devno] = dev
	c.supers[devno] = super
	return nil
}

// Clear an association between a BlockDevice/*Superblock pair and a device
// number.
func (c *LRUCache) UnmountDevice(devno int) os.Error {
	c.m.Lock()         // acquire the write mutex (+++)
	defer c.m.Unlock() // defer release of the write mutex (---)
	c.devs[devno] = nil
	c.supers[devno] = nil
	return nil
}

// GetBlock obtains a specified block from a given device. This function
// requires that the device specific is a mounted valid device, no further
// error checking is performed here.
func (c *LRUCache) GetBlock(dev, bnum int, btype BlockType, only_search int) *buf {
	c.m.Lock()         // acquire the write mutex (+++)
	defer c.m.Unlock() // defer release of the write mutex (---)

	var bp *buf

	// Search the hash chain for (dev, block). Each block number is hashed to
	// a bucket in c.buf_hash and the blocks stored there are linked via the
	// b_hash pointers in the *buf struct.
	if dev != NO_DEV {
		b := bnum & HASH_MASK
		bp = c.buf_hash[b]
		for bp != nil {
			if bp.blocknr == bnum && bp.dev == dev {
				// Block needed has been found
				if bp.count == 0 {
					c._NL_rm_lru(bp)
				}
				bp.count++
				return bp
			} else {
				// This block is not the one sought
				bp = bp.b_hash
			}
		}
	}

	// Desired block is not available on chain. Take oldest block ('front')
	bp = c.front
	if bp == nil {
		panic("all buffers in use")
	}
	c._NL_rm_lru(bp)

	// Remove the block that was just taken from its hash chain
	b := bp.blocknr & HASH_MASK
	prev_ptr := c.buf_hash[b]
	if prev_ptr == bp {
		c.buf_hash[b] = bp.b_hash
	} else {
		// The block just taken is not on the front of its hash chain
		for prev_ptr.b_hash != nil {
			if prev_ptr.b_hash == bp {
				prev_ptr.b_hash = bp.b_hash // found it
				break
			} else {
				prev_ptr = prev_ptr.b_hash // keep looking
			}
		}
	}

	// If the block taken is dirty, make it clean by writing it to the disk.
	// Avoid hysterisis by flushing all other dirty blocks for the same
	// device.
	if bp.dev != NO_DEV && bp.dirty {
		// We cannot use c.Flush(dev) here, since that requires the mutex and
		// we are currently holding it. So we refactor that function into
		// flushall(), which does not require the mutex, and then utilize that
		// in c.Flush().
		c._NL_flushall(bp.dev)
	}

	// We use the garbage collector for the actual block data, so invalidate
	// what we have here and create a new block of data. This allows us to
	// avoid lots of runtime checking to see if we already have a useable
	// block of data.

	blocksize := c.supers[dev].Block_size

	switch btype {
	case INODE_BLOCK:
		bp.block = make(InodeBlock, blocksize/V2_INODE_SIZE)
	case DIRECTORY_BLOCK:
		bp.block = make(DirectoryBlock, blocksize/V2_DIRENT_SIZE)
	case INDIRECT_BLOCK:
		bp.block = make(IndirectBlock, blocksize/4)
	case MAP_BLOCK:
		bp.block = make(MapBlock, blocksize/2)
	case FULL_DATA_BLOCK:
		bp.block = make(FullDataBlock, blocksize)
	case PARTIAL_DATA_BLOCK:
		bp.block = make(PartialDataBlock, blocksize)
	default:
		panic(fmt.Sprintf("Invalid block type specified: %d", btype))
	}

	// we avoid hard-setting count (we increment instead) and we don't reset
	// the dirty flag (since the previous flushall would have done that)
	bp.dev = dev
	bp.blocknr = bnum
	bp.count++
	b = bp.blocknr & HASH_MASK
	bp.b_hash = c.buf_hash[b]
	c.buf_hash[b] = bp

	// Go get the requested block unless searchin or prefetching
	if dev != NO_DEV {
		if only_search == PREFETCH {
			bp.dev = NO_DEV
		} else {
			if only_search == NORMAL {
				c._NL_read_block(bp) // call non-locking worker function
			}
		}
	}

	return bp
}

// Return a block to the list of available blocks. Depending on block_type it
// may be put on the front or rear of the LRU chain. Blocks that are expected
// to be needed again shortly (e.g., partially full data blocks) go on the
// rear; blocks that are unlikely to be needed again shortly (e.g., full data
// blocks) go on the front. Blocks whose loss can hurt the integrity of the
// file system (e.g., inode blocks) are written to the disk immediately if
// they are dirty.
func (c *LRUCache) put_block(bp *buf, btype BlockType) os.Error {
	if bp == nil {
		return nil
	}

	bp.count--
	if bp.count > 0 { // block is still in use
		return nil
	}

	// Put this block back on the LRU chain. If the ONE_SHOT bit is set in
	// block_type, the block is not likely to be needed again shortly, so put
	// it on the front of the LRU chain where it will be the first one to be
	// taken when a free buffer is needed later.
	if btype&ONE_SHOT > 0 {
		// Block probably won't be needed quickly. Put it on the front of the
		// chain. It will be the next block to be evicted from the cache.
		bp.prev = nil
		bp.next = c.front
		if c.front == nil {
			c.rear = bp
		} else {
			c.front.prev = bp
		}
		c.front = bp
	} else {
		// Block probably will be needed quickly. Put it on read of chain. It
		// will not be evicted from the cache for a long time.
		bp.prev = c.rear
		bp.next = nil
		if c.rear == nil {
			c.front = bp
		} else {
			c.rear.next = bp
		}
		c.rear = bp
	}

	// Some blocks are so important (e.g., inodes, indirect blocks) that they
	// should be written to the disk immediately to avoid messing up the file
	// system in the event of a crash.
	if (btype&WRITE_IMMED > 0) && bp.dirty {
		return c.WriteBlock(bp)
	}

	return nil
}

func (c *LRUCache) Invalidate(dev int) {
	c.m.Lock()
	defer c.m.Unlock()
	for i := 0; i < NR_BUFS; i++ {
		if c.buf[i].dev == dev {
			c.buf[i].dev = NO_DEV
		}
	}
}

func (c *LRUCache) Flush(dev int) {
	c.m.Lock()
	defer c.m.Unlock()
	c._NL_flushall(dev)
}

func (c *LRUCache) ReadBlock(bp *buf) os.Error {
	c.m.RLock()
	defer c.m.RUnlock()
	err := c._NL_read_block(bp)
	return err
}

func (c *LRUCache) WriteBlock(bp *buf) os.Error {
	c.m.RLock()
	defer c.m.RUnlock()
	blocksize := c.supers[bp.dev].Block_size
	pos := int64(blocksize) * int64(bp.blocknr)
	err := c.devs[bp.dev].Write(bp.block, pos)
	return err
}

// The following functions are non-locking utility functions. This is
// indicated by the _NL at the start of their names. These functions are only
// meant to be called from other functions that have already obtained the
// buffer cache mutex.

// Remove a block from its LRU chain
func (c *LRUCache) _NL_rm_lru(bp *buf) {
	nextp := bp.next
	prevp := bp.prev
	if prevp != nil {
		prevp.next = nextp
	} else {
		c.front = nextp
	}

	if nextp != nil {
		nextp.prev = prevp
	} else {
		c.rear = prevp
	}
}

// Read a block from the underlying device
func (c *LRUCache) _NL_read_block(bp *buf) os.Error {
	blocksize := c.supers[bp.dev].Block_size
	pos := int64(blocksize) * int64(bp.blocknr)
	return c.devs[bp.dev].Read(bp.block, pos)
}

// Flush all dirty blocks for one device
func (c *LRUCache) _NL_flushall(dev int) {
	// TODO: These should be static (or pre-created) so the file server can't
	// possible panic due to failed memory allocation.
	var dirty = make([]*buf, NR_BUFS) // a slice of dirty blocks
	ndirty := 0

	var bp *buf
	for i := 0; i < NR_BUFS; i++ {
		bp = c.buf[i]
		if bp.dirty && bp.dev == dev {
			log.Printf("Found a dirty block: %d", bp.blocknr)
			log.Printf("Block type: %T", bp.block)
			_debugPrintBlock(bp, c.supers[bp.dev])
			dirty[ndirty] = bp
			ndirty++
		}
	}

	// TODO: Remove this NOW
	actuallyWrite := true

	if ndirty > 0 {
		blocksize := int64(c.supers[dirty[0].dev].Block_size)
		dev := c.devs[dirty[0].dev]
		// TODO: Use the 'Scatter' method instead, if we can
		for i := 0; i < ndirty; i++ {
			bp = dirty[i]
			pos := blocksize * int64(bp.blocknr)
			if actuallyWrite {
				err := dev.Write(bp.block, pos)
				if err != nil {
					panic("something went wrong during _NL_flushall")
				}
			}
		}
		//c.devs[dev].Scatter(dirty[:ndirty]) // write the list of dirty blocks
	}
}

// Filesystem functions

func (fs *FileSystem) alloc_zone(dev int, zone int) (int, os.Error) {
	var bit uint
	var z uint
	sp := fs.supers[dev]

	// If z is 0, skip initial part of the map known to be fully in use
	if z == sp.Firstdatazone {
		bit = sp.Z_Search
	} else {
		bit = z - (sp.Firstdatazone - 1)
	}

	b := fs.alloc_bit(dev, ZMAP, bit)
	if b == NO_BIT {
		if dev == ROOT_DEVICE {
			log.Printf("No space on rootdevice %d", dev)
		} else {
			log.Printf("No space on device %d", dev)
		}
		return NO_ZONE, ENOSPC
	}
	if z == sp.Firstdatazone {
		sp.m.Lock()
		sp.Z_Search = b
		sp.m.Unlock()
	}

	return int(sp.Firstdatazone - 1 + b), nil
}
