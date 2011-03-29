package minixfs

type Block interface {
	isBlockType()
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

func (b *InodeBlock) isBlockType() {}
func (b *DirectoryBlock) isBlockType() {}
func (b *IndirectBlock) isBlockType() {}
func (b *MapBlock) isBlockType() {}
func (b *FullDataBlock) isBlockType() {}
func (b *PartialDataBlock) isBlockType() {}

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
