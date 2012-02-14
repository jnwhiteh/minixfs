package inode

import (
	. "minixfs/common"
	. "minixfs/testutils"
	"testing"
)

func TestClose(test *testing.T) {
	var bcache BlockCache = nil
	cache := NewCache(bcache, NR_DEVICES, NR_INODES).(*inodeCache)

	if err := cache.Close(); err != nil {
		ErrorHere(test, "Failed when closing icache: %s", err)
	}
	if _, ok := <-cache.in; ok {
		FatalHere(test, "icache did not close properly")
	}
	if _, ok := <-cache.out; ok {
		FatalHere(test, "icache did not close properly")
	}
}
