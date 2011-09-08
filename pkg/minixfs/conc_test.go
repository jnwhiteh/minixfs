package minixfs

import (
	"sync"
	"testing"
)

// We need a cache that will block when we need it to
type cache_lru_block struct {
	BlockCache
	blockOn map[BlockType] chan bool
	blocked map[BlockType] chan bool
}

func (c *cache_lru_block) GetBlock(dev, bnum int, btype BlockType, only_search int) *CacheBlock {
	if ch, ok := c.blockOn[btype]; ok && ch != nil {
		// notify someone that we are at the 'blocked' stage
		if bch, ok := c.blocked[btype]; ok && bch != nil {
			<-bch
		}

		<-ch
	}
	return c.BlockCache.GetBlock(dev, bnum, btype, only_search)
}

// This test checks to see that an open on a device should be able to proceed
// even if a read on another file is blocked waiting for the device. In the
// non-concurrent implementation this will deadlock, but it should pass in a
// correct implementation.
func Test_BlockedRead_Open(test *testing.T) {
	fs, proc := OpenMinix3(test)

	// block on any FULL_DATA_BLOCK reads
	blockOn := map[BlockType]chan bool {
		FULL_DATA_BLOCK: make(chan bool),
	}
	blocked := map[BlockType]chan bool {
		FULL_DATA_BLOCK: make(chan bool),
	}
	fs.cache = &cache_lru_block{fs.cache, blockOn, blocked}

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		file, err := fs.Open(proc, "/sample/europarl-en.txt", O_RDONLY, 0666)
		if err != nil {
			test.Errorf("Failed when opening file: %s - %s", err, herestr(2))
		}
		buf := make([]byte, 1024)
		file.Read(buf) // this should block
		fs.Close(proc, file)
		wg.Done()
	}()

	go func() {
		// wait until the read call is already happening
		<-blocked[FULL_DATA_BLOCK]
		file, err := fs.Open(proc, "/etc/motd", O_RDONLY, 0666)
		if err != nil {
			test.Errorf("Failed when opening file: %s - %s", err, herestr(2))
		}
		blockOn[FULL_DATA_BLOCK] <- true // release the read call
		fs.Close(proc, file)
		wg.Done()
	}()

	wg.Wait()
	fs.Exit(proc)
	fs.Shutdown()
}
