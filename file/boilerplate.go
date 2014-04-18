package file

import (
	"github.com/jnwhiteh/minixfs/common"
)

type req_File_Read struct {
	buf []byte
	pos int
}
type res_File_Read struct {
	Arg0 int
	Arg1 error
}
type req_File_Write struct {
	buf []byte
	pos int
}
type res_File_Write struct {
	Arg0 int
	Arg1 error
}
type req_File_Truncate struct {
	size int
}
type res_File_Truncate struct {
	Arg0 error
}
type req_File_Fstat struct {
}
type res_File_Fstat struct {
	Arg0 *common.StatInfo
	Arg1 error
}
type req_File_Sync struct {
}
type res_File_Sync struct {
	Arg0 error
}
type req_File_Dup struct {
}
type res_File_Dup struct {
	Arg0 common.File
}
type req_File_Close struct {
}
type res_File_Close struct {
	Arg0 error
}
type res_File_Async struct {
	ch chan resFile
}

// Interface types and implementations
type reqFile interface {
	is_reqFile()
}
type resFile interface {
	is_resFile()
}

func (r req_File_Read) is_reqFile()     {}
func (r res_File_Read) is_resFile()     {}
func (r req_File_Write) is_reqFile()    {}
func (r res_File_Write) is_resFile()    {}
func (r req_File_Truncate) is_reqFile() {}
func (r res_File_Truncate) is_resFile() {}
func (r req_File_Fstat) is_reqFile()    {}
func (r res_File_Fstat) is_resFile()    {}
func (r req_File_Sync) is_reqFile()     {}
func (r res_File_Sync) is_resFile()     {}
func (r req_File_Dup) is_reqFile()      {}
func (r res_File_Dup) is_resFile()      {}
func (r req_File_Close) is_reqFile()    {}
func (r res_File_Close) is_resFile()    {}
func (r res_File_Async) is_resFile()    {}

// Type check request/response types
var _ reqFile = req_File_Read{}
var _ resFile = res_File_Read{}
var _ reqFile = req_File_Write{}
var _ resFile = res_File_Write{}
var _ reqFile = req_File_Truncate{}
var _ resFile = res_File_Truncate{}
var _ reqFile = req_File_Fstat{}
var _ resFile = res_File_Fstat{}
var _ reqFile = req_File_Sync{}
var _ resFile = res_File_Sync{}
var _ reqFile = req_File_Dup{}
var _ resFile = res_File_Dup{}
var _ reqFile = req_File_Close{}
var _ resFile = res_File_Close{}
var _ resFile = res_File_Async{}

func (s *server_File) Read(buf []byte, pos int) (int, error) {
	s.in <- req_File_Read{buf, pos}
	ares := (<-s.out).(res_File_Async)
	result := (<-ares.ch).(res_File_Read)
	return result.Arg0, result.Arg1
}
func (s *server_File) Write(buf []byte, pos int) (int, error) {
	s.in <- req_File_Write{buf, pos}
	result := (<-s.out).(res_File_Write)
	return result.Arg0, result.Arg1
}
func (s *server_File) Truncate(size int) error {
	s.in <- req_File_Truncate{size}
	result := (<-s.out).(res_File_Truncate)
	return result.Arg0
}
func (s *server_File) Fstat() (*common.StatInfo, error) {
	s.in <- req_File_Fstat{}
	result := (<-s.out).(res_File_Fstat)
	return result.Arg0, result.Arg1
}
func (s *server_File) Sync() error {
	s.in <- req_File_Sync{}
	result := (<-s.out).(res_File_Sync)
	return result.Arg0
}
func (s *server_File) Dup() common.File {
	s.in <- req_File_Dup{}
	result := (<-s.out).(res_File_Dup)
	return result.Arg0
}
func (s *server_File) Close() error {
	s.in <- req_File_Close{}
	result := (<-s.out).(res_File_Close)
	return result.Arg0
}
