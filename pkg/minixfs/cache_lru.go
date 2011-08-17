package minixfs

import (
	"fmt"
	"log"
	"os"
)

// An elaboration of the CacheBlock type, decorated with the members we need
// to handle the LRU cache policy.
type lru_buf struct {
	*CacheBlock
	next *lru_buf // used to link all free bufs in a chain
	prev *lru_buf // used to link all free bufs the other way

	b_hash *lru_buf // used to link all bufs for a hash mask together
}

// LRUCache is a block cache implementation that will evict the least recently
// used block when necessary. The number of cached blocks is static.
type LRUCache struct {
	// These struct elements are duplicates of those that can be found in
	// the FileSystem struct. By duplicating them, we make LRUCache a
	// self-contained data structure that has a well-defined interface.
	devs   []BlockDevice // the block devices that comprise the file system
	supers []*Superblock // the superblocks for the given devices

	buf      []*lru_buf // static list of cache blocks
	buf_hash []*lru_buf // the buffer hash table

	front *lru_buf // a pointer to the least recently used block
	rear  *lru_buf // a pointer to the most recently used block

	in  chan lru_Request  // an incoming channel for requests
	out chan lru_Response // an outgoing channel for response
}

var LRU_ALLINUSE *CacheBlock = new(CacheBlock)

// NewLRUCache creates a new LRUCache with the given size
func NewLRUCache() BlockCache {
	cache := &LRUCache{
		devs:     make([]BlockDevice, NR_SUPERS),
		supers:   make([]*Superblock, NR_SUPERS),
		buf:      make([]*lru_buf, NR_BUFS),
		buf_hash: make([]*lru_buf, NR_BUF_HASH),
		in:       make(chan lru_Request),
		out:      make(chan lru_Response),
	}

	// Create all of the entries in buf ahead of time
	for i := 0; i < NR_BUFS; i++ {
		cache.buf[i] = new(lru_buf)
		cache.buf[i].CacheBlock = new(CacheBlock)
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

	// Start the main processing loop
	go cache.loop()

	return cache
}

func (c *LRUCache) loop() {
	var in <-chan lru_Request = c.in
	var out chan<- lru_Response = c.out

	for req := range in {
		switch req := req.(type) {
		case lru_mountRequest:
			err := c.mountDevice(req.devno, req.dev, req.super)
			out <- lru_errResponse{err}
		case lru_unmountRequest:
			err := c.unmountDevice(req.dev)
			out <- lru_errResponse{err}
		case lru_getRequest:
			block := c.getBlock(req.devno, req.bnum, req.btype, req.only_search)
			out <- lru_getResponse{block}
		case lru_putRequest:
			err := c.putBlock(req.cb, req.btype)
			out <- lru_errResponse{err}
		case lru_invalidateRequest:
			c.invalidate(req.dev)
			out <- lru_emptyResponse{}
		case lru_flushRequest:
			c.flush(req.dev)
			out <- lru_emptyResponse{}
		}
	}
}

func (c *LRUCache) MountDevice(devno int, dev BlockDevice, super *Superblock) os.Error {
	c.in <- lru_mountRequest{devno, dev, super}
	res := (<-c.out).(lru_errResponse)
	return res.err
}

func (c *LRUCache) UnmountDevice(devno int) os.Error {
	c.in <- lru_unmountRequest{devno}
	res := (<-c.out).(lru_errResponse)
	return res.err
}

func (c *LRUCache) GetBlock(dev, bnum int, btype BlockType, only_search int) *CacheBlock {
	c.in <- lru_getRequest{dev, bnum, btype, only_search}
	res := (<-c.out).(lru_getResponse)
	if res.cb == LRU_ALLINUSE {
		panic("all buffers in use")
	}
	return res.cb
}

func (c *LRUCache) PutBlock(cb *CacheBlock, btype BlockType) os.Error {
	c.in <- lru_putRequest{cb, btype}
	res := (<-c.out).(lru_errResponse)
	return res.err
}

func (c *LRUCache) Invalidate(dev int) {
	c.in <- lru_invalidateRequest{dev}
	<-c.out
}

func (c *LRUCache) Flush(dev int) {
	c.in <- lru_flushRequest{dev}
	<-c.out
}

// Associate a BlockDevice and *Superblock with a device number so it can be
// used internally. This operation requires the write portion of the RWMutex
// since it alters the devs and supers arrays.
func (c *LRUCache) mountDevice(devno int, dev BlockDevice, super *Superblock) os.Error {
	if c.devs[devno] != nil || c.supers[devno] != nil {
		return EBUSY
	}
	c.devs[devno] = dev
	c.supers[devno] = super
	return nil
}

// Clear an association between a BlockDevice/*Superblock pair and a device
// number.
func (c *LRUCache) unmountDevice(devno int) os.Error {
	c.devs[devno] = nil
	c.supers[devno] = nil
	return nil
}

// GetBlock obtains a specified block from a given device. This function
// requires that the device specific is a mounted valid device, no further
// error checking is performed here.
func (c *LRUCache) getBlock(dev, bnum int, btype BlockType, only_search int) *CacheBlock {
	var bp *lru_buf

	// TODO: What do we do if someone asks for NO_BLOCK

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
					c.rm_lru(bp)
				}
				bp.count++
				return bp.CacheBlock
			} else {
				// This block is not the one sought
				bp = bp.b_hash
			}
		}
	}

	// Desired block is not available on chain. Take oldest block ('front')
	bp = c.front
	if bp == nil {
		// This panic can no longer be raised here, it crashes the server.
		// Instead we need to return an error, and panic from the handler.
		return LRU_ALLINUSE
	}
	c.rm_lru(bp)

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
		c.flushall(bp.dev)
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
				c.read_block(bp) // call non-locking worker function
			}
		}
	}

	return bp.CacheBlock
}

// Return a block to the list of available blocks. Depending on block_type it
// may be put on the front or rear of the LRU chain. Blocks that are expected
// to be needed again shortly (e.g., partially full data blocks) go on the
// rear; blocks that are unlikely to be needed again shortly (e.g., full data
// blocks) go on the front. Blocks whose loss can hurt the integrity of the
// file system (e.g., inode blocks) are written to the disk immediately if
// they are dirty.
func (c *LRUCache) putBlock(cb *CacheBlock, btype BlockType) os.Error {
	if cb == nil {
		return nil
	}

	cb.count--
	if cb.count > 0 { // block is still in use
		return nil
	}

	// Find the lru_buf that corresponds to the given CacheBlock
	dev := cb.dev
	bnum := cb.blocknr

	var bp *lru_buf
	var cursor *lru_buf = c.buf_hash[bnum&HASH_MASK]
	for cursor != nil {
		if cursor.blocknr == bnum && cursor.dev == dev {
			bp = cursor
			break
		} else {
			cursor = cursor.b_hash
		}
	}

	// bp should contain the lru_buf for this CacheBlock

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

func (c *LRUCache) invalidate(dev int) {
	for i := 0; i < NR_BUFS; i++ {
		if c.buf[i].dev == dev {
			c.buf[i].dev = NO_DEV
		}
	}
}

func (c *LRUCache) flush(dev int) {
	c.flushall(dev)
}

func (c *LRUCache) WriteBlock(bp *lru_buf) os.Error {
	blocksize := c.supers[bp.dev].Block_size
	pos := int64(blocksize) * int64(bp.blocknr)
	err := c.devs[bp.dev].Write(bp.block, pos)
	return err
}

// Remove a block from its LRU chain
func (c *LRUCache) rm_lru(bp *lru_buf) {
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
func (c *LRUCache) read_block(bp *lru_buf) os.Error {
	blocksize := c.supers[bp.dev].Block_size
	pos := int64(blocksize) * int64(bp.blocknr)
	return c.devs[bp.dev].Read(bp.block, pos)
}

// Flush all dirty blocks for one device
func (c *LRUCache) flushall(dev int) {
	// TODO: These should be static (or pre-created) so the file server can't
	// possible panic due to failed memory allocation.
	var dirty = make([]*lru_buf, NR_BUFS) // a slice of dirty blocks
	ndirty := 0

	// TODO: Remove these control variables
	var _showdebug = false
	var _actuallywrite = true

	var bp *lru_buf
	for i := 0; i < NR_BUFS; i++ {
		bp = c.buf[i]
		if bp.dirty && bp.dev == dev {
			if _showdebug {
				log.Printf("Found a dirty block: %d", bp.blocknr)
				log.Printf("Block type: %T", bp.block)
				_debugPrintBlock(bp.CacheBlock, c.supers[bp.dev])
			}
			dirty[ndirty] = bp
			ndirty++
		}
	}

	if ndirty > 0 {
		blocksize := int64(c.supers[dirty[0].dev].Block_size)
		dev := c.devs[dirty[0].dev]
		// TODO: Use the 'Scatter' method instead, if we can
		for i := 0; i < ndirty; i++ {
			bp = dirty[i]
			pos := blocksize * int64(bp.blocknr)
			if _actuallywrite {
				err := dev.Write(bp.block, pos)
				if err != nil {
					panic("something went wrong during flushall")
				}
			}
		}
		//c.devs[dev].Scatter(dirty[:ndirty]) // write the list of dirty blocks
	}
}
