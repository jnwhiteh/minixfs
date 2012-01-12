package dinode

import (
	. "minixfs/common"
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

func New(inode *CacheInode, devinfo DeviceInfo, cache BlockCache) Dinode {
	dinode := &dinode{
		inode,
		devinfo,
		cache,
		make(chan m_dinode_req),
		make(chan m_dinode_res),
		new(sync.WaitGroup),
		nil,
	}

	go dinode.loop()

	return dinode
}

func (d *dinode) loop() {
	var in <-chan m_dinode_req = d.in
	var out chan<- m_dinode_res = d.out

	for req := range in {
		switch req := req.(type) {
		case m_dinode_req_lookup:
			d.waitGroup.Add(1)
			callback := make(chan m_dinode_res)
			out <- m_dinode_res_async{callback}

			go func() {
				defer close(callback)
				defer d.waitGroup.Done()

				inum := 0
				err := d.search_dir(req.name, &inum, LOOKUP)
				if err != nil {
					callback <- m_dinode_res_lookup{false, 0, 0}
				} else {
					callback <- m_dinode_res_lookup{true, d.inode.Devno, inum}
				}
			}()
		case m_dinode_req_isempty:
			// Perform this lookup asynchronously, as well
			d.waitGroup.Add(1)
			callback := make(chan m_dinode_res)
			out <- m_dinode_res_async{callback}

			go func() {
				defer close(callback)
				defer d.waitGroup.Done()

				zeroinode := 0
				if err := d.search_dir("", &zeroinode, IS_EMPTY); err != nil {
					callback <- m_dinode_res_isempty{false}
				} else {
					callback <- m_dinode_res_isempty{true}
				}
			}()
		case m_dinode_req_link:
			// Wait for any outstanding lookup requests to finish
			d.waitGroup.Wait()

			inum := req.inum
			err := d.search_dir(req.name, &inum, ENTER)
			out <- m_dinode_res_err{err}
		case m_dinode_req_unlink:
			// Wait for any outstanding lookup requests to finish
			d.waitGroup.Wait()

			inum := 0
			err := d.search_dir(req.name, &inum, DELETE)
			out <- m_dinode_res_err{err}
		case m_dinode_req_close:
			d.waitGroup.Wait()
			out <- m_dinode_res_err{nil}
			break
		}
	}

	close(d.in)
	close(d.out)
}

func (d *dinode) Lookup(name string) (bool, int, int) {
	d.in <- m_dinode_req_lookup{name}
	ares := (<-d.out).(m_dinode_res_async)
	res := (<-ares.callback).(m_dinode_res_lookup)
	return res.ok, res.devno, res.inum
}

func (d *dinode) Link(name string, inum int) error {
	d.in <- m_dinode_req_link{name, inum}
	res := (<-d.out).(m_dinode_res_err)
	return res.err
}

func (d *dinode) Unlink(name string) error {
	d.in <- m_dinode_req_unlink{name}
	res := (<-d.out).(m_dinode_res_err)
	return res.err
}

func (d *dinode) IsEmpty() bool {
	d.in <- m_dinode_req_isempty{}
	ares := (<-d.out).(m_dinode_res_async)
	res := (<-ares.callback).(m_dinode_res_isempty)
	return res.empty
}

func (d *dinode) Close() error {
	d.in <- m_dinode_req_close{}
	res := (<-d.out).(m_dinode_res_err)
	return res.err
}

var _ Dinode = &dinode{}
