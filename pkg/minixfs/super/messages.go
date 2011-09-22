package super

import (
	"os"
)

//////////////////////////////////////////////////////////////////////////////
// Messages for Superblock
//////////////////////////////////////////////////////////////////////////////

type m_super_req interface {
	is_m_super_req()
}

type m_super_res interface {
	is_m_super_res()
}

// Request types
type m_super_req_alloc_inode struct {
	mode uint16
}
type m_super_req_alloc_zone struct {
	zstart int
}
type m_super_req_free_inode struct {
	inum int
}
type m_super_req_free_zone struct {
	znum int
}
type m_super_req_close struct{}

// Response types
type m_super_res_alloc_inode struct {
	inum int
	err  os.Error
}
type m_super_res_alloc_zone struct {
	znum int
	err  os.Error
}
type m_super_res_empty struct{}
type m_super_res_err struct {
	err os.Error
}

// For type-checking
func (m m_super_req_alloc_inode) is_m_super_req() {}
func (m m_super_req_alloc_zone) is_m_super_req()  {}
func (m m_super_req_free_inode) is_m_super_req()  {}
func (m m_super_req_free_zone) is_m_super_req()   {}
func (m m_super_req_close) is_m_super_req()       {}

func (m m_super_res_alloc_inode) is_m_super_res() {}
func (m m_super_res_alloc_zone) is_m_super_res()  {}
func (m m_super_res_empty) is_m_super_res()       {}
func (m m_super_res_err) is_m_super_res()         {}

// Type assertions
var _ m_super_req = m_super_req_alloc_inode{}
var _ m_super_req = m_super_req_alloc_zone{}
var _ m_super_req = m_super_req_free_inode{}
var _ m_super_req = m_super_req_free_zone{}
var _ m_super_req = m_super_req_close{}

var _ m_super_res = m_super_res_alloc_inode{}
var _ m_super_res = m_super_res_alloc_zone{}
var _ m_super_res = m_super_res_empty{}
var _ m_super_res = m_super_res_err{}
