package minixfs

import (
	"sync"
)

// We need to ensure that access to count and pos are thread-safe, to prevent
// consistency issues within the filesystem.
type filp struct {
	mode  uint16
	flags int
	inode *Inode

	// The following fields are volatile and must be accessed under mutex,
	// using the methods defined on the struct.
	_count int
	_pos   int

	// The mutexes for the above fields
	m_count *sync.RWMutex
	m_pos   *sync.RWMutex
}

func NewFilp(mode uint16, flags int, inode *Inode, count, pos int) *filp {
	return &filp{
		mode, flags, inode, count, pos,
		new(sync.RWMutex),
		new(sync.RWMutex),
	}
}

// Return the current number of consumers of this filp
func (f *filp) Count() int {
	f.m_count.RLock()
	defer f.m_count.RUnlock()
	return f._count
}

// Sets the count field of the filp struct by a given delta, normally +1 or -1
func (f *filp) SetCountDelta(delta int) {
	f.m_count.Lock()
	defer f.m_count.Unlock()
	f._count += delta
}

// Return the current position within the file for this filp
func (f *filp) Pos() int {
	f.m_pos.RLock()
	defer f.m_pos.RUnlock()
	return f._pos
}

// Sets the pos field of the filp struct by a given delta. This is the same as
// a seek from the current position.
func (f *filp) SetPosDelta(delta int) {
	f.m_pos.Lock()
	defer f.m_pos.Unlock()
	f._pos += delta
	if f._pos < 0 {
		f._pos = 0
	}
}

// Sets the pos field of the filp struct to a given position.
func (f *filp) SetPos(pos int) {
	f.m_pos.Lock()
	defer f.m_pos.Unlock()
	if pos < 0 {
		f._pos = pos
	} else {
		f._pos = pos
	}
}
