package bcache

import (
	. "minixfs2/common"
	. "minixfs2/testutils"
	"sync"
	"testing"
)

func getDevInfo(bsize int) *DeviceInfo {
	info := new(DeviceInfo)
	info.Blocksize = 64
	return info
}

func openTestCache(test *testing.T) (BlockDevice, *LRUCache) {
	dev := NewTestDevice(test, 64, 100)
	cache := NewLRUCache(4, 10, 16)

	err := cache.MountDevice(0, dev, getDevInfo(64))
	if err != nil {
		ErrorHere(test, "Failed when mounting ramdisk device into cache: %s", err)
	}
	return dev, cache.(*LRUCache)
}

func closeTestCache(test *testing.T, dev BlockDevice, cache *LRUCache) {
	cache.Flush(0)
	err := cache.UnmountDevice(0)
	if err != nil {
		ErrorHere(test, "Failed when unmounting ramdisk device: %s", err)
	}

	//	if err = cache.Close(); err != nil {
	//		ErrorHere(test, "Failed when closing cache: %s", err)
	//	}

	//	if err = dev.Close(); err != nil {
	//		ErrorHere(test, "Failed when closing device: %s", err)
	//	}
}

// Check for proper resource cleanup when the cache is closed
// func TestClose(test *testing.T) {
// 	cache := NewLRUCache(NR_DEVICES, NR_BUFS, NR_BUF_HASH).(*LRUCache)
// 	cache.Close()
//
// 	if _, ok := <-cache.in; ok {
// 		FatalHere(test, "cache did not close properly")
// 	}
// 	if _, ok := <-cache.out; ok {
// 		FatalHere(test, "cache did not close properly")
// 	}
// }

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

func TestCacheFullPanic(test *testing.T) {
	dev, cache := openTestCache(test)

	for i := 0; i < 10; i++ {
		_ = cache.GetBlock(0, i, FULL_DATA_BLOCK, NORMAL)
	}

	done := make(chan bool)
	go func() {
		defer func() {
			if x := recover(); x == nil {
				ErrorHere(test, "Expected all buffers in use panic")
			}
			done <- true
		}()
		_ = cache.GetBlock(0, 11, FULL_DATA_BLOCK, NORMAL)
	}()

	<-done
	closeTestCache(test, dev, cache)
}

func TestGetConcurrency(test *testing.T) {
	dev, cache := openTestCache(test)
	bdev := NewBlockingDevice(NewTestDevice(test, 64, 100))
	cache.MountDevice(1, bdev, getDevInfo(64))

	// Test that reads from a normal device are not blocked by reads from a
	// broken device.
	wg := new(sync.WaitGroup)
	wg.Add(2)
	go func() {
		// Do the read on the broken device
		cb := cache.GetBlock(1, 0, FULL_DATA_BLOCK, NORMAL)
		cache.PutBlock(cb, FULL_DATA_BLOCK)
		wg.Done()
	}()

	go func() {
		// Wait for the device to be blocked
		<-bdev.HasBlocked
		cb := cache.GetBlock(0, 0, FULL_DATA_BLOCK, NORMAL)
		// Now unblock that device so we can shut down
		bdev.Unblock <- true
		cache.PutBlock(cb, FULL_DATA_BLOCK)

		wg.Done()
	}()

	wg.Wait()
	if err := cache.UnmountDevice(1); err != nil {
		ErrorHere(test, "Failed when unmounting device: %s", err)
	}
	if err := bdev.Close(); err != nil {
		ErrorHere(test, "Failed when closing device: %s", err)
	}
	closeTestCache(test, dev, cache)
}

// Test that blocks are cached. This test will deadlock if more than one block
// read is attempted from the underlying device.
func TestDoesCache(test *testing.T) {
	// Open an always-broken device
	dev := NewBlockingDevice(NewTestDevice(test, 64, 100))
	cache := NewLRUCache(4, 10, 16)
	err := cache.MountDevice(0, dev, getDevInfo(64))
	if err != nil {
		ErrorHere(test, "Failed when mounting ramdisk device into cache: %s", err)
	}

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		// Allow a single block to be read
		<-dev.HasBlocked
		dev.Unblock <- true
		wg.Done()
	}()

	go func() {
		cb1 := cache.GetBlock(0, 5, FULL_DATA_BLOCK, NORMAL)
		data, ok := cb1.Block.(FullDataBlock)
		if !ok {
			ErrorHere(test, "Did not get a FullDataBlock")
		}
		if data[0] != 5 {
			ErrorHere(test, "Data in block did not match, expected %x, got %x", 5, data[0])
		}

		// this should be pulled from the cache, not from the device
		cb2 := cache.GetBlock(0, 5, FULL_DATA_BLOCK, NORMAL)
		if cb1 != cb2 {
			ErrorHere(test, "Cache block mismatch, expected %p, got %p", cb1, cb2)
		}
		data, ok = cb2.Block.(FullDataBlock)
		if !ok {
			ErrorHere(test, "Did not get a FullDataBlock")
		}
		if data[0] != 5 {
			ErrorHere(test, "Data in block did not match, expected %x, got %x", 5, data[0])
		}

		cache.PutBlock(cb1, FULL_DATA_BLOCK)
		cache.PutBlock(cb2, FULL_DATA_BLOCK)

		wg.Done()
	}()

	wg.Wait()

	err = cache.UnmountDevice(0)
	if err != nil {
		ErrorHere(test, "Failed when unmounting ramdisk device: %s", err)
	}

	// if err = cache.Close(); err != nil {
	// 	ErrorHere(test, "Failed when closing cache: %s", err)
	// }

	// if err = dev.Close(); err != nil {
	// 	ErrorHere(test, "Failed when closing device: %s", err)
	// }
}
