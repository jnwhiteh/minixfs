package dinode

import (
	. "minixfs/common"
	"sync"
)

// The interface to the dinode is returned as a channel, with methods wrapping
// the channel operations.
type requestChan struct {
	in  chan m_dinode_req
	out chan m_dinode_res
}

// A Dinode is a process-oriented directory inode, shared amongst all open
// 'clients' of that inode. Any directory lookup/link/unlink must be made
// through a Dinode. This allows these operations to proceed concurrently for
// two distinct directory inodes.
type dinode struct {
	inode   *CacheInode
	devinfo DeviceInfo
	cache   BlockCache

	in     chan m_dinode_req
	out    chan m_dinode_res
	locked chan m_dinode_req

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
		nil,
		new(sync.WaitGroup),
		nil,
	}

	go dinode.loop()

	return &requestChan{dinode.in, dinode.out}
}

func (d *dinode) loop() {
	var in <-chan m_dinode_req = d.in
	var out chan<- m_dinode_res = d.out

	for req := range in {
		switch req := req.(type) {
		case m_dinode_req_lock:
			// Someone is asking for exclusive access to the device, so move
			// the channels around to arrange that, and return the new channel
			// interface.
			if d.locked != nil {
				// We are already locked, return nil
				out <- m_dinode_res_lock{nil}
				continue
			}
			d.locked = d.in
			dinodeLocked := &requestChan{
				make(chan m_dinode_req),
				d.out,
			}
			in = dinodeLocked.in
			out <- m_dinode_res_lock{dinodeLocked}
		case m_dinode_req_unlock:
			if d.locked == nil {
				// We are not locked, panic
				out <- m_dinode_res_unlock{false}
			}
			in = d.locked
			out <- m_dinode_res_unlock{true}
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
		case m_dinode_req_lookupget:
			d.waitGroup.Add(1)
			callback := make(chan m_dinode_res)
			out <- m_dinode_res_async{callback}

			go func() {
				defer close(callback)
				defer d.waitGroup.Done()

				inum := 0
				err := d.search_dir(req.name, &inum, LOOKUP)
				if err != nil {
					callback <- m_dinode_res_lookupget{nil, err}
				} else {
					inode, err := req.icache.GetInode(d.inode.Devno, inum)
					callback <- m_dinode_res_lookupget{inode, err}
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

//////////////////////////////////////////////////////////////////////////////
// Public interface, provided as a requestChan
//////////////////////////////////////////////////////////////////////////////
func (d *requestChan) Lookup(name string) (bool, int, int) {
	d.in <- m_dinode_req_lookup{name}
	ares := (<-d.out).(m_dinode_res_async)
	res := (<-ares.callback).(m_dinode_res_lookup)
	return res.ok, res.devno, res.inum
}

func (d *requestChan) LookupGet(name string, icache InodeCache) (*CacheInode, error) {
	d.in <- m_dinode_req_lookupget{name, icache}
	ares := (<-d.out).(m_dinode_res_async)
	res := (<-ares.callback).(m_dinode_res_lookupget)
	return res.inode, res.err
}

func (d *requestChan) Link(name string, inum int) error {
	d.in <- m_dinode_req_link{name, inum}
	res := (<-d.out).(m_dinode_res_err)
	return res.err
}

func (d *requestChan) Unlink(name string) error {
	d.in <- m_dinode_req_unlink{name}
	res := (<-d.out).(m_dinode_res_err)
	return res.err
}

func (d *requestChan) IsEmpty() bool {
	d.in <- m_dinode_req_isempty{}
	ares := (<-d.out).(m_dinode_res_async)
	res := (<-ares.callback).(m_dinode_res_isempty)
	return res.empty
}

func (d *requestChan) Lock() Dinode {
	d.in <- m_dinode_req_lock{}
	res := (<-d.out).(m_dinode_res_lock)
	return res.dinode
}

func (d *requestChan) Unlock() {
	d.in <- m_dinode_req_unlock{}
	res := (<-d.out).(m_dinode_res_unlock)
	if !res.ok {
		panic("Unlocked a non-locked Dinode")
	}
}

func (d *requestChan) Close() error {
	d.in <- m_dinode_req_close{}
	res := (<-d.out).(m_dinode_res_err)
	return res.err
}

var _ Dinode = &requestChan{}
