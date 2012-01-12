package bitmap

//////////////////////////////////////////////////////////////////////////////
// Messages for Superblock
//////////////////////////////////////////////////////////////////////////////

type m_bitmap_req interface {
	is_m_bitmap_req()
}

type m_bitmap_res interface {
	is_m_bitmap_res()
}

// Request types
type m_bitmap_req_alloc_inode struct {
}
type m_bitmap_req_alloc_zone struct {
	zstart int
}
type m_bitmap_req_free_inode struct {
	inum int
}
type m_bitmap_req_free_zone struct {
	znum int
}
type m_bitmap_req_close struct{}

// Response types
type m_bitmap_res_alloc_inode struct {
	inum int
	err  error
}
type m_bitmap_res_alloc_zone struct {
	znum int
	err  error
}
type m_bitmap_res_empty struct{}
type m_bitmap_res_err struct {
	err error
}

// For type-checking
func (m m_bitmap_req_alloc_inode) is_m_bitmap_req() {}
func (m m_bitmap_req_alloc_zone) is_m_bitmap_req()  {}
func (m m_bitmap_req_free_inode) is_m_bitmap_req()  {}
func (m m_bitmap_req_free_zone) is_m_bitmap_req()   {}
func (m m_bitmap_req_close) is_m_bitmap_req()       {}

func (m m_bitmap_res_alloc_inode) is_m_bitmap_res() {}
func (m m_bitmap_res_alloc_zone) is_m_bitmap_res()  {}
func (m m_bitmap_res_empty) is_m_bitmap_res()       {}
func (m m_bitmap_res_err) is_m_bitmap_res()         {}

// Type assertions
var _ m_bitmap_req = m_bitmap_req_alloc_inode{}
var _ m_bitmap_req = m_bitmap_req_alloc_zone{}
var _ m_bitmap_req = m_bitmap_req_free_inode{}
var _ m_bitmap_req = m_bitmap_req_free_zone{}
var _ m_bitmap_req = m_bitmap_req_close{}

var _ m_bitmap_res = m_bitmap_res_alloc_inode{}
var _ m_bitmap_res = m_bitmap_res_alloc_zone{}
var _ m_bitmap_res = m_bitmap_res_empty{}
var _ m_bitmap_res = m_bitmap_res_err{}
