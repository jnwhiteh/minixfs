package minixfs

import (
	"os"
)

// Device messages

type m_dev_req struct {
	call CallNumber
	buf  interface{}
	pos  int64
}

type m_dev_res struct {
	err os.Error
}

// Cache messages

type m_cache_req interface {
	is_m_cache_req()
}

type m_cache_res interface {
	is_m_cache_res()
}

type m_cache_req_mount struct {
	devno int
	dev   BlockDevice
	super *Superblock
}

type m_cache_req_get struct {
	devno       int
	bnum        int
	btype       BlockType
	only_search int
}

type m_cache_req_put struct {
	cb    *CacheBlock
	btype BlockType
}

type m_cache_req_unmount struct{ dev int }
type m_cache_req_invalidate struct{ dev int }
type m_cache_req_flush struct{ dev int }

type m_cache_res_err struct {
	err os.Error
}

type m_cache_res_get struct {
	cb *CacheBlock
}

type m_cache_res_empty struct{}

func (req m_cache_req_mount) is_m_cache_req()      {}
func (req m_cache_req_get) is_m_cache_req()        {}
func (req m_cache_req_put) is_m_cache_req()        {}
func (req m_cache_req_unmount) is_m_cache_req()    {}
func (req m_cache_req_invalidate) is_m_cache_req() {}
func (req m_cache_req_flush) is_m_cache_req()      {}

func (res m_cache_res_err) is_m_cache_res()   {}
func (res m_cache_res_get) is_m_cache_res()   {}
func (res m_cache_res_empty) is_m_cache_res() {}

// type assertions for devices
var _ m_cache_req = m_cache_req_mount{}
var _ m_cache_req = m_cache_req_get{}
var _ m_cache_req = m_cache_req_put{}
var _ m_cache_req = m_cache_req_unmount{}
var _ m_cache_req = m_cache_req_invalidate{}
var _ m_cache_req = m_cache_req_flush{}

var _ m_cache_res = m_cache_res_err{}
var _ m_cache_res = m_cache_res_get{}
var _ m_cache_res = m_cache_res_empty{}
