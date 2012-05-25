package filp

import (
	. "minixfs2/common"
)

type server_Filp struct {
	count int  // the number of clients sharing this position
	pos   int  // the current position in the file
	file  File // the file backing this

	in  chan reqFilp
	out chan resFilp
}

func NewFilp(file File) Filp {
	filp := &server_Filp{
		1, // begin with one client, dup and close add/remove
		0, // always begin at the start of file
		file,
		make(chan reqFilp),
		make(chan resFilp),
	}

	go filp.loop()
	return filp
}

func (filp *server_Filp) loop() {
	alive := true
	for alive {
		req := <-filp.in
		switch req := req.(type) {
		case req_Filp_Seek:
			switch req.whence {
			case 1:
				filp.pos += req.pos
			case 0:
				filp.pos = req.pos
			default:
				panic("NYI: Seek with whence > 1")
			}
			filp.out <- res_Filp_Seek{filp.pos, nil}
		case req_Filp_Read:
			n, err := filp.file.Read(req.buf, filp.pos)
			filp.pos += n
			filp.out <- res_Filp_Read{n, err}
		case req_Filp_Write:
			n, err := filp.file.Write(req.buf, filp.pos)
			filp.pos += n
			filp.out <- res_Filp_Write{n, err}
		case req_Filp_Dup:
			filp.count++
			filp.file.Dup() // notify the file that it has one more client
			filp.out <- res_Filp_Dup{filp}
		case req_Filp_Close:
			filp.count--
			if filp.count == 0 {
				alive = false // this position no longer in-use, so kill it
			}
			filp.file.Close() // notify the file that it has one fewer client
			filp.out <- res_Filp_Close{}
		}
	}
}
