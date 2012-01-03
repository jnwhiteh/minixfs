package bcache

import (
	"fmt"
	"log"
	. "../../minixfs/common/_obj/minixfs/common"
	debug "../../minixfs/debug/_obj/minixfs/debug"
	"os"
	"sync"
)

// An elaboration of the CacheBlock type, decorated with the members we need
// to handle the LRU cache policy.
type lru_buf struct {
	*CacheBlock
	next *lru_buf // used to link all free bufs in a chain
	prev *lru_buf // used to link all free bufs the other way

	b_hash *lru_buf // used to link all bufs for a hash mask together

	waiting []chan m_cache_res_block // a list of waiting get requests
	m       *sync.Mutex              // mutex for the waiting slice
}

// LRUCache is a block cache implementation that will evict the least recently
// used block when necessary. The number of cached blocks is static.
type LRUCache struct {
	// These struct elements are duplicates of those that can be found in
	// the FileSystem struct. By duplicating them, we make LRUCache a
	// self-contained data structure that has a well-defined interface.
	devs    []RandDevice // the block devices that comprise the file system
	devinfo []DeviceInfo // the information/parameters for the device

	buf       []*lru_buf // static list of cache blocks
	buf_hash  []*lru_buf // the buffer hash table
	hash_mask int        // the mask for entries in the buffer hash table

	front *lru_buf // a pointer to the least recently used block
	rear  *lru_buf // a pointer to the most recently used block

	in  chan m_cache_req // an incoming channel for requests
	out chan m_cache_res // an outgoing channel for response

	// TODO: Remove these flags
	showdebug     bool // on flush, show the blocks that have changed.
	actuallywrite bool // commit dirty blocks back to the device
}

var LRU_ALLINUSE *CacheBlock = new(CacheBlock)

// NewLRUCache creates a new LRUCache with the given size
func NewLRUCache(numdevices int, numslots int, numhash int) BlockCache {
	cache := &LRUCache{
		devs:          make([]RandDevice, numdevices),
		devinfo:       make([]DeviceInfo, numdevices),
		buf:           make([]*lru_buf, numslots),
		buf_hash:      make([]*lru_buf, numhash),
		in:            make(chan m_cache_req),
		out:           make(chan m_cache_res),
		showdebug:     false,
		actuallywrite: false,
	}

	// Create all of the entries in buf ahead of time
	for i := 0; i < numslots; i++ {
		cache.buf[i] = new(lru_buf)
		cache.buf[i].CacheBlock = new(CacheBlock)
		cache.buf[i].Devno = NO_DEV
		cache.buf[i].m = new(sync.Mutex)
	}

	for i := 1; i < numslots-1; i++ {
		cache.buf[i].prev = cache.buf[i-1]
		cache.buf[i].next = cache.buf[i+1]
	}

	cache.front = cache.buf[0]
	cache.front.next = cache.buf[1]

	cache.rear = cache.buf[numslots-1]
	cache.rear.prev = cache.buf[numslots-2]

	for i := 0; i < numslots-1; i++ {
		cache.buf[i].b_hash = cache.buf[i].next
	}

	cache.buf_hash[0] = cache.front
	cache.hash_mask = numhash - 1

	// Start the main processing loop
	go cache.loop()

	return cache
}

func (c *LRUCache) loop() {
	var in <-chan m_cache_req = c.in
	var out chan<- m_cache_res = c.out

	for req := range in {
		switch req := req.(type) {
		case m_cache_req_mount:
			err := c.mountDevice(req.devno, req.dev, req.devinfo)
			out <- m_cache_res_err{err}
		case m_cache_req_unmount:
			err := c.unmountDevice(req.dev)
			out <- m_cache_res_err{err}
		case m_cache_req_get:
			callback := make(chan m_cache_res_block)

			// search for the desired block in the cache
			var bp *lru_buf
			if req.devno != NO_DEV {
				b := req.bnum & c.hash_mask
				for bp = c.buf_hash[b]; bp != nil; bp = bp.b_hash {
					if bp.Blockno == req.bnum && bp.Devno == req.devno {
						// we found what we were looking for!
						break
					}
				}
			}

			if bp != nil && bp.Blockno == req.bnum && bp.Devno == req.devno {
				bp.m.Lock()
				if len(bp.waiting) > 0 {
					// this block is being loaded asynchronously, join the
					// waiting list
					bp.waiting = append(bp.waiting, callback)
					bp.m.Unlock()
					// the server will become available for another request,
					// and this request will be finished when the block has
					// been loaded.
				} else {
					// the block is ready now, so return it
					bp.m.Unlock()
					out <- m_cache_res_async_block{callback}
					callback <- m_cache_res_block{bp.CacheBlock}
				}
			} else {
				// We will need to load the block from the backing store,
				// asynchronously. Any get requests performed during this load
				// should be blocked and woken in FIFO order of the original
				// request.
				bp := c.evictBlock()
				if bp == nil {
					// LRU_ALLINUSE happened
					out <- m_cache_res_async_block{callback}
					callback <- m_cache_res_block{LRU_ALLINUSE}
				} else {
					bp.m.Lock()
					bp.waiting = append(bp.waiting, callback)
					bp.m.Unlock()

					out <- m_cache_res_async_block{callback}

					// perform a load of this block asynchronously
					go func() {
						c.loadBlock(bp, req.devno, req.bnum, req.btype, req.only_search)
						bp.m.Lock()
						for _, callback := range bp.waiting {
							callback <- m_cache_res_block{bp.CacheBlock}
						}
						bp.waiting = nil
						bp.m.Unlock()
					}()
				}
			}
		case m_cache_req_put:
			err := c.putBlock(req.cb, req.btype)
			out <- m_cache_res_err{err}
		case m_cache_req_invalidate:
			c.invalidate(req.dev)
			out <- m_cache_res_empty{}
		case m_cache_req_flush:
			c.flush(req.dev)
			out <- m_cache_res_empty{}
		case m_cache_req_close:
			out <- m_cache_res_err{nil}
			close(c.in)
			close(c.out)
		}
	}
}

func (c *LRUCache) MountDevice(devno int, dev RandDevice, info DeviceInfo) os.Error {
	c.in <- m_cache_req_mount{devno, dev, info}
	res := (<-c.out).(m_cache_res_err)
	return res.err
}

func (c *LRUCache) UnmountDevice(devno int) os.Error {
	c.in <- m_cache_req_unmount{devno}
	res := (<-c.out).(m_cache_res_err)
	return res.err
}

func (c *LRUCache) GetBlock(dev, bnum int, btype BlockType, only_search int) *CacheBlock {
	c.in <- m_cache_req_get{dev, bnum, btype, only_search, false}
	ares := (<-c.out).(m_cache_res_async_block)
	res := <-ares.ch
	if res.cb == LRU_ALLINUSE {
		panic("all buffers in use")
	}
	return res.cb
}

func (c *LRUCache) PutBlock(cb *CacheBlock, btype BlockType) os.Error {
	c.in <- m_cache_req_put{cb, btype}
	res := (<-c.out).(m_cache_res_err)
	return res.err
}

func (c *LRUCache) Invalidate(dev int) {
	c.in <- m_cache_req_invalidate{dev}
	<-c.out
}

func (c *LRUCache) Flush(dev int) {
	c.in <- m_cache_req_flush{dev}
	<-c.out
}

func (c *LRUCache) Close() os.Error {
	c.in <- m_cache_req_close{}
	res := (<-c.out).(m_cache_res_err)
	return res.err
}

//////////////////////////////////////////////////////////////////////////////
// Private implementations
//////////////////////////////////////////////////////////////////////////////

// Associate a BlockDevice and a blocksize with a device number so it can be
// used internally.
func (c *LRUCache) mountDevice(devno int, dev RandDevice, info DeviceInfo) os.Error {
	if c.devs[devno] != nil {
		return EBUSY
	}
	c.devs[devno] = dev
	c.devinfo[devno] = info
	return nil
}

// Clear an association between a BlockDevice/*Superblock pair and a device
// number.
func (c *LRUCache) unmountDevice(devno int) os.Error {
	c.devs[devno] = nil
	return nil
}

func (c *LRUCache) evictBlock() *lru_buf {
	var bp *lru_buf

	// Desired block is not available on chain. Take oldest block ('front')
	bp = c.front
	if bp == nil {
		// This panic can no longer be raised here, it crashes the server.
		// Instead we need to return an error, and panic from the handler.
		return nil
	}
	c.rm_lru(bp)

	// Remove the block that was just taken from its hash chain
	b := bp.Blockno & c.hash_mask
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
	if bp.Devno != NO_DEV && bp.Dirty {
		c.flush(bp.Devno)
	}

	return bp
}

// loadBlock loads a specified block from a given device into the buffer slot
// 'bp'. This function requires that the specified device is a valid device,
// as no further error checking is performed here.
func (c *LRUCache) loadBlock(bp *lru_buf, dev, bnum int, btype BlockType, only_search int) os.Error {
	// We use the garbage collector for the actual block data, so invalidate
	// what we have here and create a new block of data. This allows us to
	// avoid lots of runtime checking to see if we already have a useable
	// block of data.

	blocksize := c.devinfo[dev].Blocksize

	switch btype {
	case INODE_BLOCK:
		bp.Block = make(InodeBlock, blocksize/V2_INODE_SIZE)
	case DIRECTORY_BLOCK:
		bp.Block = make(DirectoryBlock, blocksize/V2_DIRENT_SIZE)
	case INDIRECT_BLOCK:
		bp.Block = make(IndirectBlock, blocksize/4)
	case MAP_BLOCK:
		bp.Block = make(MapBlock, blocksize/2)
	case FULL_DATA_BLOCK:
		bp.Block = make(FullDataBlock, blocksize)
	case PARTIAL_DATA_BLOCK:
		bp.Block = make(PartialDataBlock, blocksize)
	default:
		panic(fmt.Sprintf("Invalid block type specified: %d", btype))
	}

	// we avoid hard-setting count (we increment instead) and we don't reset
	// the dirty flag (since the previous flushall would have done that)
	bp.Devno = dev
	bp.Blockno = bnum
	bp.Count++
	b := bp.Blockno & c.hash_mask
	bp.b_hash = c.buf_hash[b]
	c.buf_hash[b] = bp
	bp.Buf = bp

	// Go get the requested block unless searching or prefetching
	if dev != NO_DEV {
		if only_search == PREFETCH {
			bp.Devno = NO_DEV
		} else {
			if only_search == NORMAL {
				pos := int64(blocksize) * int64(bnum)

				// This read needs to be performed asynchronously.
				err := c.devs[bp.Devno].Read(bp.Block, pos)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
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

	cb.Count--
	if cb.Count > 0 { // block is still in use
		return nil
	}

	// We can find the lru_buf that corresponds to the given CacheBlock by
	// checking the 'buf' field and coercing it.
	bp := cb.Buf.(*lru_buf)

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
		// Block probably will be needed quickly. Put it on rear of chain. It
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
	if (btype&WRITE_IMMED > 0) && bp.Dirty {
		return c.WriteBlock(bp)
	}

	return nil
}

func (c *LRUCache) invalidate(dev int) {
	for i := 0; i < NR_BUFS; i++ {
		if c.buf[i].Devno == dev {
			c.buf[i].Devno = NO_DEV
		}
	}
}

func (c *LRUCache) flush(dev int) {
	// TODO: These should be static (or pre-created) so the file server can't
	// possible panic due to failed memory allocation.
	var dirty = make([]*lru_buf, NR_BUFS) // a slice of dirty blocks
	ndirty := 0

	// TODO: Remove this debug code
	var bp *lru_buf
	for i := 0; i < NR_BUFS; i++ {
		bp = c.buf[i]
		if bp.Dirty && bp.Devno == dev {
			if c.showdebug {
				log.Printf("Found a dirty block: %d", bp.Blockno)
				log.Printf("Block type: %T", bp.Block)
				debug.PrintBlock(bp.CacheBlock, c.devinfo[bp.Devno])
			}
			dirty[ndirty] = bp
			ndirty++
		}
	}

	if ndirty > 0 {
		blocksize := int64(c.devinfo[dirty[0].Devno].Blocksize)
		dev := c.devs[dirty[0].Devno]
		// TODO: Use the 'Scatter' method instead, if we can
		for i := 0; i < ndirty; i++ {
			bp = dirty[i]
			pos := blocksize * int64(bp.Blockno)
			if c.actuallywrite {
				err := dev.Write(bp.Block, pos)
				if err != nil {
					panic("something went wrong during flushall")
				}
			}
		}
		//c.devs[dev].Scatter(dirty[:ndirty]) // write the list of dirty blocks
	}
}

func (c *LRUCache) WriteBlock(bp *lru_buf) os.Error {
	blocksize := c.devinfo[bp.Devno].Blocksize
	pos := int64(blocksize) * int64(bp.Blockno)
	err := c.devs[bp.Devno].Write(bp.Block, pos)
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

var _ BlockCache = &LRUCache{}
