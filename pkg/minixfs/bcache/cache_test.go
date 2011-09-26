package bcache

import (
	. "../../minixfs/common/_obj/minixfs/common"
	. "../../minixfs/device/_obj/minixfs/device"
	. "../../minixfs/testutils/_obj/minixfs/testutils"
	"testing"
)

func openTestCache(test *testing.T) (RandDevice, *LRUCache) {
	bsize := 64
	data := make([]byte, bsize * 100)
	for i := 0; i < 100; i ++ {
		for j := 0; j < 64; j++ {
			data[(i * 64) + j] = byte(i)
		}
	}
	dev, err := NewRamdiskDevice(data)
	if err != nil {
		ErrorHere(test, "Failed when opening ramdisk device: %s", err)
	}
	cache := NewLRUCache(1, 10, 16)
	err = cache.MountDevice(0, dev, DeviceInfo{0, 64})
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

	closeTestCache(test, dev, cache)
}
