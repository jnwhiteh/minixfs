package minixfs

type Block interface {
	setBlockNum(uint) // set the block number
	setDirty(bool)    // set the dirty flag
	isDirty() bool    // returns the dirty flag
	blockNum() uint   // returns the block number
}

// TODO: Unepxort the 'data' member

type InodeBlock struct {
	Data []disk_inode // block containing a series of inodes
	buf  *Buf
}

type DirectoryBlock struct {
	Data []Directory // block containing directory entries
	buf  *Buf
}

type IndirectBlock struct {
	Data []uint32 // block containing 32-bit zone numbers
	buf  *Buf
}

type MapBlock struct {
	Data []uint16 // block containing bitmaps (in 16-bit chunks)
	buf  *Buf
}

type FullDataBlock struct {
	Data []uint8 // block containing data (in bytes)
	buf  *Buf
}

type PartialDataBlock struct {
	Data []uint8 // block containing data (in bytes)
	buf  *Buf
}

// Implement the setBlockNum method for each block type
func (b *InodeBlock) setBlockNum(num uint)       { b.buf.num = num }
func (b *DirectoryBlock) setBlockNum(num uint)   { b.buf.num = num }
func (b *IndirectBlock) setBlockNum(num uint)    { b.buf.num = num }
func (b *MapBlock) setBlockNum(num uint)         { b.buf.num = num }
func (b *FullDataBlock) setBlockNum(num uint)    { b.buf.num = num }
func (b *PartialDataBlock) setBlockNum(num uint) { b.buf.num = num }

// Implement the setDirty method for each block type
func (b *InodeBlock) setDirty(dirty bool)       { b.buf.dirty = dirty }
func (b *DirectoryBlock) setDirty(dirty bool)   { b.buf.dirty = dirty }
func (b *IndirectBlock) setDirty(dirty bool)    { b.buf.dirty = dirty }
func (b *MapBlock) setDirty(dirty bool)         { b.buf.dirty = dirty }
func (b *FullDataBlock) setDirty(dirty bool)    { b.buf.dirty = dirty }
func (b *PartialDataBlock) setDirty(dirty bool) { b.buf.dirty = dirty }

// Implement the isDirty method for each block type
func (b *InodeBlock) isDirty() bool       { return b.buf.dirty }
func (b *DirectoryBlock) isDirty() bool   { return b.buf.dirty }
func (b *IndirectBlock) isDirty() bool    { return b.buf.dirty }
func (b *MapBlock) isDirty() bool         { return b.buf.dirty }
func (b *FullDataBlock) isDirty() bool    { return b.buf.dirty }
func (b *PartialDataBlock) isDirty() bool { return b.buf.dirty }

// Implement the blockNum method for each block type
func (b *InodeBlock) blockNum() uint       { return b.buf.num }
func (b *DirectoryBlock) blockNum() uint   { return b.buf.num }
func (b *IndirectBlock) blockNum() uint    { return b.buf.num }
func (b *MapBlock) blockNum() uint         { return b.buf.num }
func (b *FullDataBlock) blockNum() uint    { return b.buf.num }
func (b *PartialDataBlock) blockNum() uint { return b.buf.num }


// Ensure each block type implements the Block interface
var _ Block = &InodeBlock{nil, nil}
var _ Block = &DirectoryBlock{nil, nil}
var _ Block = &IndirectBlock{nil, nil}
var _ Block = &MapBlock{nil, nil}
var _ Block = &FullDataBlock{nil, nil}
var _ Block = &PartialDataBlock{nil, nil}

func (fs *FileSystem) NewInodeBlock() *InodeBlock {
	return &InodeBlock{make([]disk_inode, fs.super.inodes_per_block), new(Buf)}
}

func (fs *FileSystem) NewDirectoryBlock() *DirectoryBlock {
	return &DirectoryBlock{make([]Directory, fs.Block_size/64), new(Buf)}
}

func (fs *FileSystem) NewIndirectBlock() *IndirectBlock {
	return &IndirectBlock{make([]uint32, fs.Block_size/4), new(Buf)}
}

func (fs *FileSystem) NewMapBlock() *MapBlock {
	return &MapBlock{make([]uint16, FS_BITMAP_CHUNKS(fs.Block_size)), new(Buf)}
}

func (fs *FileSystem) NewFullDataBlock() *FullDataBlock {
	return &FullDataBlock{make([]byte, fs.Block_size), new(Buf)}
}

func (fs *FileSystem) NewPartialDataBlock() *PartialDataBlock {
	return &PartialDataBlock{make([]byte, fs.Block_size), new(Buf)}
}
