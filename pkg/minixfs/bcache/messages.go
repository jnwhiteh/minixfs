package bcache

import (
	. "../../minixfs/common/_obj/minixfs/common"
	"os"
)

//////////////////////////////////////////////////////////////////////////////
// Messages for BlockCache interface
//////////////////////////////////////////////////////////////////////////////

type m_cache_req interface {
	is_m_cache_req()
}

type m_cache_res interface {
	is_m_cache_res()
}

type m_cache_req_mount struct {
	devno   int
	dev     RandDevice
	devinfo DeviceInfo
}
type m_cache_req_get struct {
	devno       int
	bnum        int
	btype       BlockType
	only_search int
	offset      bool
}
type m_cache_req_put struct {
	cb    *CacheBlock
	btype BlockType
}
type m_cache_req_unmount struct{ dev int }
type m_cache_req_reserve struct {
	dev  int
	bnum int
}
type m_cache_req_claim struct {
	dev   int
	bnum  int
	btype BlockType
}
type m_cache_req_invalidate struct{ dev int }
type m_cache_req_flush struct{ dev int }
type m_cache_req_close struct{}

type m_cache_res_err struct {
	err os.Error
}
type m_cache_res_block struct {
	cb *CacheBlock
}
type m_cache_res_async_block struct {
	ch chan m_cache_res_block
}
type m_cache_res_reserve struct {
	avail bool
}
type m_cache_res_empty struct{}

func (req m_cache_req_mount) is_m_cache_req()      {}
func (req m_cache_req_get) is_m_cache_req()        {}
func (req m_cache_req_put) is_m_cache_req()        {}
func (req m_cache_req_unmount) is_m_cache_req()    {}
func (req m_cache_req_reserve) is_m_cache_req()    {}
func (req m_cache_req_claim) is_m_cache_req()      {}
func (req m_cache_req_invalidate) is_m_cache_req() {}
func (req m_cache_req_flush) is_m_cache_req()      {}
func (req m_cache_req_close) is_m_cache_req()      {}

func (res m_cache_res_err) is_m_cache_res()         {}
func (res m_cache_res_block) is_m_cache_res()       {}
func (res m_cache_res_async_block) is_m_cache_res() {}
func (res m_cache_res_reserve) is_m_cache_res()     {}
func (res m_cache_res_empty) is_m_cache_res()       {}

// type assertions for devices
var _ m_cache_req = m_cache_req_mount{}
var _ m_cache_req = m_cache_req_get{}
var _ m_cache_req = m_cache_req_put{}
var _ m_cache_req = m_cache_req_unmount{}
var _ m_cache_req = m_cache_req_reserve{}
var _ m_cache_req = m_cache_req_claim{}
var _ m_cache_req = m_cache_req_invalidate{}
var _ m_cache_req = m_cache_req_flush{}
var _ m_cache_req = m_cache_req_close{}

var _ m_cache_res = m_cache_res_err{}
var _ m_cache_res = m_cache_res_block{}
var _ m_cache_res = m_cache_res_async_block{}
var _ m_cache_res = m_cache_res_reserve{}
var _ m_cache_res = m_cache_res_empty{}
