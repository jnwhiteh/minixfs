package minixfs

import "fmt"
import "os"

// The buffer cache in the Minix file system is built on top of the raw device
// and provides access to recently requested blocks served from memory. In
// this implementation, this is just a limited length hash. Since Go is
// garbage collected, we do not have to worry about keeping static buffers of
// blocks, we just remove all references to a block when we wish to remove it
// and can freely allocate a new one.
//
// When a block is obtained via 'GetBlock' its count field is incremented and
// marks it in-use. When a block is returned using 'PutBlock', this same field
// is decremented. Only if this field is 0 is the block actually moved to the
// list of blocks available for eviction.
//
// For simplicity, each block cache is tied to a single device.

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
}

// LRUCache is a block cache implementation that will evict the least recently
// used block when necessary. The number of cached blocks is static.
type LRUCache struct {
	fs    *FileSystem    // the filesystem for which this is a cache
	bhash []map[int]*buf // a hash from block number to buf

	size     int // the current size of the cache, in blocks
	max_size int // the maximum size of the cache, in blocks

	front *buf // a pointer to the least recently used block
	rear  *buf // a pointer to the most recently used block
}

// NewLRUCache creates a new LRUCache with the given size
func NewLRUCache(fs *FileSystem, size int) *LRUCache {
	cache := &LRUCache{
		fs:       fs,
		bhash:    make([]map[int]*buf, NR_SUPERS),
		size:     0,
		max_size: size,
		front:    nil,
		rear:     nil,
	}

	// TODO: A hashmap won't ever compact itself, so we can end up using more
	// actual space than if we had just kept a static buffer with a hash chain
	// as in the original implementation.
	for i := 0; i < NR_SUPERS; i++ {
		cache.bhash[i] = make(map[int]*buf)
	}
	return cache
}

func (c *LRUCache) GetBlock(dev, bnum int, btype BlockType, onlySearch bool) *buf {
	hash := c.bhash[dev]
	// Check to see if the block is cached
	if buf, ok := hash[bnum]; ok {
		if buf.count == 0 {
			c.rm_lru(buf) // remove the block from the free list
			buf.count++
			c.size++
			return buf
		}
	}

	// Block was not found, check if we need to evict a block
	if c.size >= c.max_size {
		if c.front == nil {
			// TODO: Is this the right behaviour?
			panic(fmt.Sprintf("cache: all buffers in use: %d", c.size))
		} else {
			// Evict the block on the front of the LRU chain
			bp := c.front
			hash[bp.blocknr] = nil, false
			c.rm_lru(bp)
			c.size--

			// If the block being evicted is dirty, make it clean by writing
			// it to the disk. Avoid hysterisis by flushing all other dirty
			// blocks for this device.
			if bp.dirty {
				c.Flush(bp.dev)
			}
		}
	}

	// Allocate a new block and fetch the data from the device
	bp := new(buf)
	blocksize := int(c.fs.supers[dev].Block_size)

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

	bp.dev = dev
	bp.blocknr = bnum
	bp.count = 1
	bp.dirty = false
	hash[bnum] = bp
	c.size++

	if onlySearch {
		panic("NYI: LRUCache.GetBlock with onlySearch = true")
	}

	c.ReadBlock(bp)
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
		// Block properly will be needed quickly. Put it on read of chain. It
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
	c.bhash[dev] = make(map[int]*buf)
	c.front = nil
	c.rear = nil
}

func (c *LRUCache) Flush(dev int) {
	dirty := make([]*buf, 0) // a slice of dirty blocks
	ndirty := 0

	for _, bp := range c.bhash[dev] {
		if bp.dirty {
			dirty = append(dirty, bp)
			ndirty++
		}
	}
	if len(dirty) > 0 {
		c.fs.devs[dev].Scatter(dirty) // write the list of blocks, scattered
	}
}

// rm_lru removes a block from its LRU chain
func (c *LRUCache) rm_lru(bp *buf) {
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

func (c *LRUCache) ReadBlock(bp *buf) os.Error {
	blocksize := c.fs.supers[bp.dev].Block_size
	pos := int64(blocksize) * int64(bp.blocknr)
	return c.fs.devs[bp.dev].Read(bp.block, pos)
}

func (c *LRUCache) WriteBlock(bp *buf) os.Error {
	blocksize := c.fs.supers[bp.dev].Block_size
	pos := int64(blocksize) * int64(bp.blocknr)
	return c.fs.devs[bp.dev].Write(bp.block, pos)
}
