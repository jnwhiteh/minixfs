package minixfs

// We need to ensure that access to count and pos are thread-safe, to prevent
// consistency issues within the filesystem.
type filp struct {
	mode  uint16
	flags int
	inode *Inode
	count int
	pos   int
}

func NewFilp(mode uint16, flags int, inode *Inode, count, pos int) *filp {
	return &filp{
		mode, flags, inode, count, pos,
	}
}

// Return the current number of consumers of this filp
func (f *filp) Count() int {
	return f.count
}

// Sets the count field of the filp struct by a given delta, normally +1 or -1
func (f *filp) SetCountDelta(delta int) {
	f.count += delta
}

// Return the current position within the file for this filp
func (f *filp) Pos() int {
	return f.pos
}

// Sets the pos field of the filp struct by a given delta. This is the same as
// a seek from the current position.
func (f *filp) SetPosDelta(delta int) {
	f.pos += delta
	if f.pos < 0 {
		f.pos = 0
	}
}

// Sets the pos field of the filp struct to a given position.
func (f *filp) SetPos(pos int) {
	if pos < 0 {
		f.pos = pos
	} else {
		f.pos = pos
	}
}
