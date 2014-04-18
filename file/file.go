package file

import (
	"github.com/jnwhiteh/minixfs/common"
	"sync"
)

type server_File struct {
	rip   *common.Inode   // the underlying inode
	count int             // the number of clients of this server
	wg    *sync.WaitGroup // tracking outstanding read requests

	in  chan reqFile
	out chan resFile
}

func NewFile(rip *common.Inode) common.File {
	file := &server_File{
		rip,
		1,
		new(sync.WaitGroup),
		make(chan reqFile),
		make(chan resFile),
	}

	go file.loop()
	return file
}

func (file *server_File) loop() {
	alive := true
	for alive {
		req := <-file.in
		switch req := req.(type) {
		case req_File_Read:
			// Indicate we have another outstanding reader
			file.wg.Add(1)
			callback := make(chan resFile)
			file.out <- res_File_Async{callback}

			// Launch a new goroutine to perform the read, using the callback
			// channel to return the result.
			go func() {
				n, err := common.Read(file.rip, req.buf, req.pos)
				callback <- res_File_Read{n, err}
				file.wg.Done() // signal completion
			}()
		case req_File_Write:
			file.wg.Wait() // wait for any outstanding reads to complete before proceeding
			n, err := common.Write(file.rip, req.buf, req.pos)
			file.out <- res_File_Write{n, err}
		case req_File_Truncate:
			file.wg.Wait() // wait for any outstanding reads to complete before proceeding
			common.Truncate(file.rip, req.size, file.rip.Bcache)
			file.out <- res_File_Truncate{}
		case req_File_Fstat:
			// Code here
		case req_File_Sync:
			// Code here
		case req_File_Dup:
			file.count++
			file.out <- res_File_Dup{}
		case req_File_Close:
			file.wg.Wait() // wait for any outstanding reads to complete before proeceding
			file.count--

			// Let's push our changes to the inode cache
			file.rip.Icache.FlushInode(file.rip)
			file.rip.Icache.PutInode(file.rip)

			if file.count == 0 {
				alive = false
			}

			file.out <- res_File_Close{}
		}
	}
}

var _ common.File = &server_File{}
