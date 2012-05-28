package alloctbl

type req_AllocTbl_AllocInode struct {
}
type res_AllocTbl_AllocInode struct {
	Arg0 int
	Arg1 error
}
type req_AllocTbl_AllocZone struct {
	zstart int
}
type res_AllocTbl_AllocZone struct {
	Arg0 int
	Arg1 error
}
type req_AllocTbl_FreeInode struct {
	inum int
}
type res_AllocTbl_FreeInode struct {
	Arg0 error
}
type req_AllocTbl_FreeZone struct {
	znum int
}
type res_AllocTbl_FreeZone struct {
	Arg0 error
}
type req_AllocTbl_Shutdown struct {}
type res_AllocTbl_Shutdown struct {
	Arg0 error
}

// Interface types and implementations
type reqAllocTbl interface {
	is_reqAllocTbl()
}
type resAllocTbl interface {
	is_resAllocTbl()
}

func (r req_AllocTbl_AllocInode) is_reqAllocTbl() {}
func (r res_AllocTbl_AllocInode) is_resAllocTbl() {}
func (r req_AllocTbl_AllocZone) is_reqAllocTbl()  {}
func (r res_AllocTbl_AllocZone) is_resAllocTbl()  {}
func (r req_AllocTbl_FreeInode) is_reqAllocTbl()  {}
func (r res_AllocTbl_FreeInode) is_resAllocTbl()  {}
func (r req_AllocTbl_FreeZone) is_reqAllocTbl()   {}
func (r res_AllocTbl_FreeZone) is_resAllocTbl()   {}
func (r req_AllocTbl_Shutdown) is_reqAllocTbl()   {}
func (r res_AllocTbl_Shutdown) is_resAllocTbl()   {}

// Type check request/response types
var _ reqAllocTbl = req_AllocTbl_AllocInode{}
var _ resAllocTbl = res_AllocTbl_AllocInode{}
var _ reqAllocTbl = req_AllocTbl_AllocZone{}
var _ resAllocTbl = res_AllocTbl_AllocZone{}
var _ reqAllocTbl = req_AllocTbl_FreeInode{}
var _ resAllocTbl = res_AllocTbl_FreeInode{}
var _ reqAllocTbl = req_AllocTbl_FreeZone{}
var _ resAllocTbl = res_AllocTbl_FreeZone{}
var _ reqAllocTbl = req_AllocTbl_Shutdown{}
var _ resAllocTbl = res_AllocTbl_Shutdown{}

func (s *server_AllocTbl) AllocInode() (int, error) {
	s.in <- req_AllocTbl_AllocInode{}
	result := (<-s.out).(res_AllocTbl_AllocInode)
	return result.Arg0, result.Arg1
}
func (s *server_AllocTbl) AllocZone(zstart int) (int, error) {
	s.in <- req_AllocTbl_AllocZone{zstart}
	result := (<-s.out).(res_AllocTbl_AllocZone)
	return result.Arg0, result.Arg1
}
func (s *server_AllocTbl) FreeInode(inum int) error {
	s.in <- req_AllocTbl_FreeInode{inum}
	result := (<-s.out).(res_AllocTbl_FreeInode)
	return result.Arg0
}
func (s *server_AllocTbl) FreeZone(znum int) error {
	s.in <- req_AllocTbl_FreeZone{znum}
	result := (<-s.out).(res_AllocTbl_FreeZone)
	return result.Arg0
}
func (s *server_AllocTbl) Shutdown() error {
	s.in <- req_AllocTbl_Shutdown{}
	result := (<-s.out).(res_AllocTbl_Shutdown)
	return result.Arg0
}
