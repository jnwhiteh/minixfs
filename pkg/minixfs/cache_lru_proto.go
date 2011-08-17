package minixfs

import (
	"os"
)

// Implementation of protocols for lru cache

type lru_Request interface {
	isLRURequest()
}

type lru_Response interface {
	isLRUResponse()
}

type lru_mountRequest struct {
	devno int
	dev   BlockDevice
	super *Superblock
}

type lru_getRequest struct {
	devno       int
	bnum        int
	btype       BlockType
	only_search int
}

type lru_putRequest struct {
	cb    *CacheBlock
	btype BlockType
}

type lru_unmountRequest struct{ dev int }
type lru_invalidateRequest struct{ dev int }
type lru_flushRequest struct{ dev int }

type lru_errResponse struct {
	err os.Error
}

type lru_getResponse struct {
	cb *CacheBlock
}

type lru_emptyResponse struct{}

func (req lru_mountRequest) isLRURequest()      {}
func (req lru_getRequest) isLRURequest()        {}
func (req lru_putRequest) isLRURequest()        {}
func (req lru_unmountRequest) isLRURequest()    {}
func (req lru_invalidateRequest) isLRURequest() {}
func (req lru_flushRequest) isLRURequest()      {}
func (res lru_errResponse) isLRUResponse()      {}
func (res lru_getResponse) isLRUResponse()      {}
func (res lru_emptyResponse) isLRUResponse()    {}

// Type assertions require allocations, so comment out
// var _ lru_Request = lru_mountRequest{}
// var _ lru_Request = lru_getRequest{}
// var _ lru_Request = lru_putRequest{}
// var _ lru_Request = lru_unmountRequest{}
// var _ lru_Request = lru_invalidateRequest{}
// var _ lru_Request = lru_flushRequest{}
// var _ lru_Response = lru_errResponse{}
// var _ lru_Response = lru_getResponse{}
// var _ lru_Response = lru_emptyResponse{}
