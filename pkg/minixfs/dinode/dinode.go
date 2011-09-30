package dinode

import (
	. "../../minixfs/common/_obj/minixfs/common"
	"os"
	"sync"
)

// A Dinode is a process-oriented directory inode, shared amongst all open
// 'clients' of that inode. Any directory lookup/link/unlink must be made
// through a Dinode. This allows these operations to proceed concurrently for
// two distinct directory inodes.
type dinode struct {
	inode   *CacheInode
	devinfo DeviceInfo
	cache   BlockCache

	in  chan m_dinode_req
	out chan m_dinode_res

	waitGroup *sync.WaitGroup // used for mutual exclusion for writes
	closed    chan bool
}


func (d *dinode) loop() {
	var in <-chan m_dinode_req = d.in
	var out chan<- m_dinode_res = d.out

	_ = out

	for req := range in {
		switch req := req.(type) {
		case m_dinode_req_lookup:
		case m_dinode_req_link:
		case m_dinode_req_unlink:
		case m_dinode_req_close:
		}
	}
}

func (d *dinode) Lookup(name string) (bool, int, int) {
	d.in <- m_dinode_req_lookup{name}
	res := (<-d.out).(m_dinode_res_lookup)
	return res.ok, res.devno, res.inum
}

func (d *dinode) Link(name string, inum int) os.Error {
	d.in <- m_dinode_req_link{name, inum}
	res := (<-d.out).(m_dinode_res_err)
	return res.err
}

func (d *dinode) Unlink(name string) os.Error {
	d.in <- m_dinode_req_unlink{name}
	res := (<-d.out).(m_dinode_res_err)
	return res.err
}

func (d *dinode) Close() os.Error {
	d.in <- m_dinode_req_close{}
	res := (<-d.out).(m_dinode_res_err)
	return res.err
}

var _ Dinode = &dinode{}
