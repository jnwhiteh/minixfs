package dinode

import (
	"os"
)

//////////////////////////////////////////////////////////////////////////////
// Messages for Dinode
//////////////////////////////////////////////////////////////////////////////

type m_dinode_req interface {
	is_m_dinode_req()
}

type m_dinode_res interface {
	is_m_dinode_res()
}

// Request types
type m_dinode_req_lookup struct {
	name string
}

type m_dinode_req_link struct {
	name string
	inum int
}

type m_dinode_req_close struct {}

type m_dinode_req_unlink struct{
	name string
}

// Response types
type m_dinode_res_lookup struct {
	ok bool
	devno int
	inum int
}

type m_dinode_res_err struct {
	err os.Error
}

// For type-checking
func (m m_dinode_req_lookup) is_m_dinode_req()  {}
func (m m_dinode_req_link) is_m_dinode_req() {}
func (m m_dinode_req_unlink) is_m_dinode_req() {}
func (m m_dinode_req_close) is_m_dinode_req() {}

func (m m_dinode_res_lookup) is_m_dinode_res()      {}
func (m m_dinode_res_err) is_m_dinode_res() {}

// Check interface implementation
var _ m_dinode_req = m_dinode_req_lookup{}
var _ m_dinode_req = m_dinode_req_link{}
var _ m_dinode_req = m_dinode_req_unlink{}
var _ m_dinode_req = m_dinode_req_close{}

var _ m_dinode_res = m_dinode_res_lookup{}
var _ m_dinode_res = m_dinode_res_err{}
