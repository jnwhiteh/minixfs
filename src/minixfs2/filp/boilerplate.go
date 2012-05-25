package filp

import (
	. "minixfs2/common"
)

type req_Filp_Seek struct {
	pos, whence int
}
type res_Filp_Seek struct {
	Arg0 int
	Arg1 error
}
type req_Filp_Read struct {
	buf []byte
}
type res_Filp_Read struct {
	Arg0 int
	Arg1 error
}
type req_Filp_Write struct {
	buf []byte
}
type res_Filp_Write struct {
	Arg0 int
	Arg1 error
}
type req_Filp_Dup struct {
}
type res_Filp_Dup struct {
	Arg0 Filp
}
type req_Filp_Close struct {
}
type res_Filp_Close struct {
	Arg0 error
}

// Interface types and implementations
type reqFilp interface {
	is_reqFilp()
}
type resFilp interface {
	is_resFilp()
}
func (r req_Filp_Seek) is_reqFilp() {}
func (r res_Filp_Seek) is_resFilp() {}
func (r req_Filp_Read) is_reqFilp() {}
func (r res_Filp_Read) is_resFilp() {}
func (r req_Filp_Write) is_reqFilp() {}
func (r res_Filp_Write) is_resFilp() {}
func (r req_Filp_Dup) is_reqFilp() {}
func (r res_Filp_Dup) is_resFilp() {}
func (r req_Filp_Close) is_reqFilp() {}
func (r res_Filp_Close) is_resFilp() {}

// Type check request/response types
var _ reqFilp = req_Filp_Seek{}
var _ resFilp = res_Filp_Seek{}
var _ reqFilp = req_Filp_Read{}
var _ resFilp = res_Filp_Read{}
var _ reqFilp = req_Filp_Write{}
var _ resFilp = res_Filp_Write{}
var _ reqFilp = req_Filp_Dup{}
var _ resFilp = res_Filp_Dup{}
var _ reqFilp = req_Filp_Close{}
var _ resFilp = res_Filp_Close{}

func (s *server_Filp) Seek(pos, whence int) (int, error) {
	s.in <- req_Filp_Seek{pos, whence}
	result := (<-s.out).(res_Filp_Seek)
	return result.Arg0, result.Arg1
}
func (s *server_Filp) Read(buf []byte) (int, error) {
	s.in <- req_Filp_Read{buf}
	result := (<-s.out).(res_Filp_Read)
	return result.Arg0, result.Arg1
}
func (s *server_Filp) Write(buf []byte) (int, error) {
	s.in <- req_Filp_Write{buf}
	result := (<-s.out).(res_Filp_Write)
	return result.Arg0, result.Arg1
}
func (s *server_Filp) Dup() (Filp) {
	s.in <- req_Filp_Dup{}
	result := (<-s.out).(res_Filp_Dup)
	return result.Arg0
}
func (s *server_Filp) Close() (error) {
	s.in <- req_Filp_Close{}
	result := (<-s.out).(res_Filp_Close)
	return result.Arg0
}
