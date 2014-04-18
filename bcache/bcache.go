package bcache

import (
	"fmt"
	"log"
	"github.com/jnwhiteh/minixfs/common"
	"github.com/jnwhiteh/minixfs/debug"
	"sync"
)

// An elaboration of the CacheBlock type, decorated with the members we need
// to handle the LRU cache policy.
type lru_buf struct {
	*common.CacheBlock

	count int      // the number of clients of this block
	next  *lru_buf // used to link all free bufs in a chain
	prev  *lru_buf // used to link all free bufs the other way

	b_hash *lru_buf // used to link all bufs for a hash mask together

	waiting []chan resBlockCache // a list of waiting get requests
	m       *sync.Mutex          // mutex for the waiting slice
}

var LRU_ALLINUSE *common.CacheBlock = new(common.CacheBlock)

type LRUCache struct {
	devices []common.BlockDevice
	devinfo []*common.DeviceInfo

	buf       []*lru_buf // static list of cache blocks
	buf_hash  []*lru_buf // the buffer hash table
	hash_mask int        // the mask for entries in the buffer hash table
	front     *lru_buf   // a pointer to the least recently used block
	rear      *lru_buf   // a pointer to the most recently used block

	in  chan reqBlockCache
	out chan resBlockCache

	showdebug     bool
	actuallywrite bool
}

// NewLRUCache creates a new LRUCache with the given size
func NewLRUCache(numdevices int, numslots int, numhash int) common.BlockCache {
	cache := &LRUCache{
		devices:  make([]common.BlockDevice, numdevices),
		devinfo:  make([]*common.DeviceInfo, numdevices),
		buf:      make([]*lru_buf, numslots),
		buf_hash: make([]*lru_buf, numhash),
		in:       make(chan reqBlockCache),
		out:      make(chan resBlockCache),
	}

	// Create all of the entries in buf ahead of time
	for i := 0; i < numslots; i++ {
		cache.buf[i] = new(lru_buf)
		cache.buf[i].CacheBlock = new(common.CacheBlock)
		cache.buf[i].Devnum = common.NO_DEV
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

	cache.showdebug = false
	cache.actuallywrite = false

	// Start the main processing loop
	go cache.loop()
	return cache
}

func (c *LRUCache) loop() {
	alive := true
	for alive {
		req := <-c.in
		switch req := req.(type) {
		case req_BlockCache_MountDevice:
			if c.devices[req.devnum] != nil {
				c.out <- res_BlockCache_MountDevice{common.EBUSY}
				continue
			}
			c.devices[req.devnum] = req.dev
			c.devinfo[req.devnum] = req.info
			c.out <- res_BlockCache_MountDevice{nil}
		case req_BlockCache_UnmountDevice:
			c.flush(req.devnum)
			c.devices[req.devnum] = nil
			c.out <- res_BlockCache_UnmountDevice{}
		case req_BlockCache_GetBlock:
			callback := make(chan resBlockCache)

			// search for the desired block in the cache
			var bp *lru_buf
			if req.devnum != common.NO_DEV {
				b := req.bnum & c.hash_mask
				for bp = c.buf_hash[b]; bp != nil; bp = bp.b_hash {
					if bp.Blocknum == req.bnum && bp.Devnum == req.devnum {
						// we found what we were looking for!
						break
					}
				}
			}

			if bp != nil && bp.Blocknum == req.bnum && bp.Devnum == req.devnum {
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
					c.out <- res_BlockCache_Async{callback}
					callback <- res_BlockCache_GetBlock{bp.CacheBlock}
				}
			} else {
				// We will need to load the block from the backing store,
				// asynchronously. Any get requests performed during this load
				// should be blocked and woken in FIFO order of the original
				// request.
				bp := c.evictBlock()
				if bp == nil {
					// LRU_ALLINUSE happened
					c.out <- res_BlockCache_Async{callback}
					callback <- res_BlockCache_GetBlock{LRU_ALLINUSE}
				} else {
					bp.m.Lock()
					bp.waiting = append(bp.waiting, callback)
					bp.m.Unlock()

					c.out <- res_BlockCache_Async{callback}

					// perform a load of this block asynchronously
					go func() {
						c.loadBlock(bp, req.devnum, req.bnum, req.btype, req.only_search)
						bp.m.Lock()
						for _, callback := range bp.waiting {
							callback <- res_BlockCache_GetBlock{bp.CacheBlock}
						}
						bp.waiting = nil
						bp.m.Unlock()
					}()
				}
			}
		case req_BlockCache_PutBlock:
			err := c.putBlock(req.cb, req.btype)
			c.out <- res_BlockCache_PutBlock{err}
		case req_BlockCache_Invalidate:
			c.invalidate(req.devnum)
			c.out <- res_BlockCache_Invalidate{}
		case req_BlockCache_Flush:
			c.flush(req.devnum)
			c.out <- res_BlockCache_Flush{}
		case req_BlockCache_Shutdown:
			for i := 0; i < len(c.devices); i++ {
				if c.devices[i] != nil {
					c.out <- res_BlockCache_Shutdown{common.EBUSY}
					continue
				}
			}
			c.out <- res_BlockCache_Shutdown{nil}
			alive = false
		}
	}
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
	b := bp.Blocknum & c.hash_mask
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
	if bp.Devnum != common.NO_DEV && bp.Dirty {
		c.flush(bp.Devnum)
	}

	return bp
}

// loadBlock loads a specified block from a given device into the buffer slot
// 'bp'. This function requires that the specified device is a valid device,
// as no further error checking is performed here.
func (c *LRUCache) loadBlock(bp *lru_buf, dev, bnum int, btype common.BlockType, only_search int) error {
	// We use the garbage collector for the actual block data, so invalidate
	// what we have here and create a new block of data. This allows us to
	// avoid lots of runtime checking to see if we already have a useable
	// block of data.

	blocksize := c.devinfo[dev].Blocksize

	switch btype {
	case common.INODE_BLOCK:
		bp.Block = make(common.InodeBlock, blocksize/common.V2_INODE_SIZE)
	case common.DIRECTORY_BLOCK:
		bp.Block = make(common.DirectoryBlock, blocksize/common.V2_DIRENT_SIZE)
	case common.INDIRECT_BLOCK:
		bp.Block = make(common.IndirectBlock, blocksize/4)
	case common.MAP_BLOCK:
		bp.Block = make(common.MapBlock, blocksize/2)
	case common.FULL_DATA_BLOCK:
		bp.Block = make(common.FullDataBlock, blocksize)
	case common.PARTIAL_DATA_BLOCK:
		bp.Block = make(common.PartialDataBlock, blocksize)
	default:
		panic(fmt.Sprintf("Invalid block type specified: %d", btype))
	}

	// we avoid hard-setting count (we increment instead) and we don't reset
	// the dirty flag (since the previous flushall would have done that)
	bp.Devnum = dev
	bp.Blocknum = bnum
	bp.count++
	b := bp.Blocknum & c.hash_mask
	bp.b_hash = c.buf_hash[b]
	c.buf_hash[b] = bp
	bp.Buf = bp

	// Go get the requested block unless searching or prefetching
	if dev != common.NO_DEV {
		if only_search == common.PREFETCH {
			bp.Devnum = common.NO_DEV
		} else {
			if only_search == common.NORMAL {
				pos := int64(blocksize) * int64(bnum)

				// This read needs to be performed asynchronously.
				err := c.devices[bp.Devnum].Read(bp.Block, pos)
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
func (c *LRUCache) putBlock(cb *common.CacheBlock, btype common.BlockType) error {
	if cb == nil {
		return nil
	}

	// We can find the lru_buf that corresponds to the given CacheBlock by
	// checking the 'buf' field and coercing it.
	bp := cb.Buf.(*lru_buf)

	bp.count--
	if bp.count > 0 { // block is still in use
		return nil
	}

	// Put this block back on the LRU chain. If the ONE_SHOT bit is set in
	// block_type, the block is not likely to be needed again shortly, so put
	// it on the front of the LRU chain where it will be the first one to be
	// taken when a free buffer is needed later.
	if btype&common.ONE_SHOT > 0 {
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
	if (btype&common.WRITE_IMMED > 0) && bp.Dirty {
		blocksize := c.devinfo[bp.Devnum].Blocksize
		pos := int64(blocksize) * int64(bp.Blocknum)
		err := c.devices[bp.Devnum].Write(bp.Block, pos)
		return err
	}

	return nil
}

func (c *LRUCache) invalidate(dev int) {
	for i := 0; i < common.NR_BUFS; i++ {
		if c.buf[i].Devnum == dev {
			c.buf[i].Devnum = common.NO_DEV
		}
	}
}

func (c *LRUCache) flush(dev int) {
	// TODO: These should be static (or pre-created) so the file server can't
	// possible panic due to failed memory allocation.
	var dirty []*lru_buf
	ndirty := 0

	// TODO: Remove this debug code
	var bp *lru_buf
	for i := 0; i < len(c.buf); i++ {
		bp = c.buf[i]
		if bp.Dirty && bp.Devnum == dev {
			if c.showdebug {
				log.Printf("Found a dirty block: %d", bp.Blocknum)
				log.Printf("Block type: %T", bp.Block)
				debug.PrintBlock(bp.CacheBlock, c.devinfo[bp.Devnum])
			}
			dirty = append(dirty, bp)
			ndirty++
		}
	}

	if ndirty > 0 {
		blocksize := int64(c.devinfo[dirty[0].Devnum].Blocksize)
		dev := c.devices[dirty[0].Devnum]
		// TODO: Use the 'Scatter' method instead, if we can
		for i := 0; i < ndirty; i++ {
			bp = dirty[i]
			pos := blocksize * int64(bp.Blocknum)
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
