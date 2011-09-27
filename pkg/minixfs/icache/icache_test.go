package icache

import (
	. "../../minixfs/common/_obj/minixfs/common"
	. "../../minixfs/testutils/_obj/minixfs/testutils"
	"testing"
)

func TestClose(test *testing.T) {
	var bcache BlockCache = nil
	icache := NewInodeCache(bcache, NR_SUPERS, NR_INODES).(*inodeCache)

	if err := icache.Close(); err != nil {
		ErrorHere(test, "Failed when closing icache: %s", err)
	}
	if _, ok := <-icache.in; ok {
		FatalHere(test, "icache did not close properly")
	}
	if _, ok := <-icache.out; ok {
		FatalHere(test, "icache did not close properly")
	}
}
