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
}
