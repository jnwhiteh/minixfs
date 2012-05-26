package file

import (
	. "minixfs2/common"
	"sync"
)

type server_File struct {
	rip *Inode
	wg  *sync.WaitGroup

	in  chan reqFile
	out chan resFile
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
				n, err := Read(file.rip, req.buf, req.pos)
				callback <- res_File_Read{n, err}
				file.wg.Done() // signal completion
			}()
		case req_File_Write:
			file.wg.Wait() // wait for any outstanding reads to complete before proceeding
			n, err := Write(file.rip, req.buf, req.pos)
			file.out <- res_File_Write{n, err}
		case req_File_Truncate:
			// Code here
		case req_File_Fstat:
			// Code here
		case req_File_Sync:
			// Code here
		case req_File_Dup:
			// Code here
		case req_File_Close:
			// Code here
		}
	}
}

var _ File = &server_File{}
