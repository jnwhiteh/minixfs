package minixfs

import (
	"encoding/binary"
	"testing"
)

// Some basic tests for the block cache.
func TestCache(test *testing.T) {
	dev, err := NewFileDevice("../../minix3usr.img", binary.LittleEndian)
	if err != nil {
		test.Fatalf("Failed to create device for minix3usr.img: %s", err)
	}

	super, err := ReadSuperblock(dev)
	if err != nil {
		test.Fatalf("Could not read superblock: %s", err)
	}

	cache := NewLRUCache()
	cache.MountDevice(0, dev, super)

	bp := cache.GetBlock(0, 0, FULL_DATA_BLOCK, NORMAL)
	cache.PutBlock(bp, FULL_DATA_BLOCK)

	bp2 := cache.GetBlock(0, 0, FULL_DATA_BLOCK, NORMAL)
	if bp != bp2 {
		test.Errorf("Blcok not cached, got %p, expected %p", bp2, bp)
	}
	cache.PutBlock(bp, FULL_DATA_BLOCK)

	orig := make([]*CacheBlock, NR_BUFS)

	// Fill the cache
	for i := 0; i < NR_BUFS; i++ {
		orig[i] = cache.GetBlock(0, i, FULL_DATA_BLOCK, NORMAL)
	}

	// Check and make sure that no block got overwritten due to cache
	for i := 0; i < NR_BUFS; i++ {
		if orig[i].dev != 0 || orig[i].blocknr != i {
			test.Errorf("Incorrect block, got (%d,%d), expected (%d,%d)", orig[i].dev, orig[i].blocknr, 0, i)
		}
	}

	// Test to ensure that we get an "All buffers in use" panic
	hadPanic := make(chan bool)
	go func() {
		defer func() {
			r := recover()
			hadPanic <- (r != nil)
		}()
		foo := cache.GetBlock(0, NR_BUFS, FULL_DATA_BLOCK, NORMAL)
		cache.PutBlock(foo, FULL_DATA_BLOCK)
	}()

	if !(<-hadPanic) {
		test.Fatalf("Expected 'all buffers in use' panic, got none")
	}
	close(hadPanic)

	// Dump all of the cache block, without actually releasing them from our
	// array.
	for i := 0; i < NR_BUFS; i++ {
		cache.PutBlock(orig[i], FULL_DATA_BLOCK)
	}

	// Request another NR_BUFS blocks (all different). We should see every
	// block be overwritten.
	diff := make([]*CacheBlock, NR_BUFS)
	for i := 0; i < NR_BUFS; i++ {
		diff[i] = cache.GetBlock(0, NR_BUFS+i, FULL_DATA_BLOCK, NORMAL)
	}

	// NONE of the blocks in 'orig' should be correct, now.
	// Check and make sure that no block got overwritten due to cache
	for i := 0; i < NR_BUFS; i++ {
		if orig[i].dev == 0 && orig[i].blocknr == i {
			test.Errorf("Incorrect block, got (%d,%d) expected something different", orig[i].dev, orig[i].blocknr)
		}
	}

	// Put back 10 blocks and verify that they are used in reverse order
	// (least recently used first).
	for i := 0; i < 10; i++ {
		cache.PutBlock(diff[i], FULL_DATA_BLOCK)
	}
	for i := 0; i < 10; i++ {
		bp := cache.GetBlock(0, i, FULL_DATA_BLOCK, NORMAL)
		if bp != diff[i] {
			test.Errorf("Expected re-use of %p, got %p", diff[i], bp)
		}
	}

	// Invalidate the cache, and ensure that we have nothing valid
	cache.Invalidate(0)

	for i := 0; i < NR_BUFS; i++ {
		if orig[i].dev != NO_DEV {
			test.Errorf("After invalidation, orig[i] with dev %d", orig[i].dev)
		}
		if diff[i].dev != NO_DEV {
			test.Errorf("After invalidation, diff[i] with dev %d", diff[i].dev)
		}
	}
}
