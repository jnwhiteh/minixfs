package bcache

import (
	. "../../minixfs/common/_obj/minixfs/common"
	. "../../minixfs/testutils/_obj/minixfs/testutils"
	"testing"
)

func openTestCache(test *testing.T) (RandDevice, *LRUCache) {
	dev := NewTestDevice(test, 64, 100)
	cache := NewLRUCache(4, 10, 16)
	err := cache.MountDevice(0, dev, DeviceInfo{0, 64})
	if err != nil {
		ErrorHere(test, "Failed when mounting ramdisk device into cache: %s", err)
	}
	return dev, cache.(*LRUCache)
}

func closeTestCache(test *testing.T, dev RandDevice, cache *LRUCache) {
	err := cache.UnmountDevice(0)
	if err != nil {
		ErrorHere(test, "Failed when unmounting ramdisk device: %s", err)
	}

	if err = cache.Close(); err != nil {
		ErrorHere(test, "Failed when closing cache: %s", err)
	}

	if err = dev.Close(); err != nil {
		ErrorHere(test, "Failed when closing device: %s", err)
	}
}

// Check for proper resource cleanup when the cache is closed
func TestClose(test *testing.T) {
	cache := NewLRUCache(NR_SUPERS, NR_BUFS, NR_BUF_HASH).(*LRUCache)
	cache.Close()

	if _, ok := <-cache.in; ok {
		FatalHere(test, "cache did not close properly")
	}
	if _, ok := <-cache.out; ok {
		FatalHere(test, "cache did not close properly")
	}
}

// Test to ensure that blocks are re-used in last-recently-used order, i.e.
// in the reverse order they are 'put' back into the cache.
func TestLRUOrder(test *testing.T) {
	dev, cache := openTestCache(test)

	// get 10 blocks
	blocks := make([]*CacheBlock, 10)
	for i := 0; i < 10; i++ {
		blocks[i] = cache.GetBlock(0, i, FULL_DATA_BLOCK, NORMAL)
	}

	// put them back
	for i := 0; i < 10; i++ {
		cache.PutBlock(blocks[i], FULL_DATA_BLOCK)
	}

	// now fetch 10 more different blocks
	blocks2 := make([]*CacheBlock, 10)
	for i := 0; i < 10; i++ {
		blocks2[i] = cache.GetBlock(0, i+10, FULL_DATA_BLOCK, NORMAL)
		if blocks2[i] != blocks[0+i] {
			ErrorHere(test, "cache block mismatch, expected %p, got %p", blocks[9-i], blocks2[i])
		}
	}

	closeTestCache(test, dev, cache)
}
