package icache

import (
	. "../../minixfs/common/_obj/minixfs/common"
	"os"
)

//////////////////////////////////////////////////////////////////////////////
// InodeCache Messages
//////////////////////////////////////////////////////////////////////////////

type m_icache_req interface {
	is_m_icache_req()
}

type m_icache_res interface {
	is_m_icache_res()
}

// Request types
type m_icache_req_updatedevinfo struct {
	devno int
	info  DeviceInfo
}
type m_icache_req_getinode struct {
	devno int
	inum  int
}
type m_icache_req_putinode struct {
	rip *CacheInode
}
type m_icache_req_isbusy struct {
	devno int
}
type m_icache_req_close struct{}

// Response types
type m_icache_res_empty struct{}
type m_icache_res_async struct {
	ch chan m_icache_res
}
type m_icache_res_getinode struct {
	rip *CacheInode
	err os.Error
}
type m_icache_res_isbusy struct {
	busy bool
}
type m_icache_res_err struct {
	err os.Error
}

// For interface implementations
func (m m_icache_req_updatedevinfo) is_m_icache_req() {}
func (m m_icache_req_getinode) is_m_icache_req()      {}
func (m m_icache_req_putinode) is_m_icache_req()      {}
func (m m_icache_req_isbusy) is_m_icache_req()        {}
func (m m_icache_req_close) is_m_icache_req()         {}

func (m m_icache_res_empty) is_m_icache_res()    {}
func (m m_icache_res_async) is_m_icache_res()    {}
func (m m_icache_res_getinode) is_m_icache_res() {}
func (m m_icache_res_isbusy) is_m_icache_res()   {}
func (m m_icache_res_err) is_m_icache_res()      {}

// Type assertions
var _ m_icache_req = m_icache_req_updatedevinfo{}
var _ m_icache_req = m_icache_req_getinode{}
var _ m_icache_req = m_icache_req_putinode{}
var _ m_icache_req = m_icache_req_isbusy{}
var _ m_icache_req = m_icache_req_close{}

var _ m_icache_res = m_icache_res_empty{}
var _ m_icache_res = m_icache_res_async{}
var _ m_icache_res = m_icache_res_getinode{}
var _ m_icache_res = m_icache_res_isbusy{}
var _ m_icache_res = m_icache_res_err{}
