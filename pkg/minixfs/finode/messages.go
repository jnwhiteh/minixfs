package minixfs

import (
	"os"
)

//////////////////////////////////////////////////////////////////////////////
// Messages for Finode
//////////////////////////////////////////////////////////////////////////////

type m_finode_req interface {
	is_m_finode_req()
}

type m_finode_res interface {
	is_m_finode_res()
}

// Request types
type m_finode_req_read struct {
	buf []byte
	pos int
}

type m_finode_req_write struct {
	buf []byte
	pos int
}

type m_finode_req_close struct{}

// Response types
type m_finode_res_io struct {
	n   int
	err os.Error
}

type m_finode_res_asyncio struct {
	callback <-chan m_finode_res_io
}

type m_finode_res_empty struct{}

// For type-checking
func (m m_finode_req_read) is_m_finode_req()  {}
func (m m_finode_req_write) is_m_finode_req() {}
func (m m_finode_req_close) is_m_finode_req() {}

func (m m_finode_res_io) is_m_finode_res()      {}
func (m m_finode_res_asyncio) is_m_finode_res() {}
func (m m_finode_res_empty) is_m_finode_res()   {}

// Check interface implementation
var _ m_finode_req = m_finode_req_read{}
var _ m_finode_req = m_finode_req_write{}
var _ m_finode_req = m_finode_req_close{}

var _ m_finode_res = m_finode_res_io{}
var _ m_finode_res = m_finode_res_asyncio{}
var _ m_finode_res = m_finode_res_empty{}
