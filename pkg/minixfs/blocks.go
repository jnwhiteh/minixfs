package minixfs

type Block interface {
	isBlockType()
}

type InodeBlock []disk_inode      // block containing a series of inodes
type DirectoryBlock []disk_dirent // block containing directory entries
type IndirectBlock []uint32       // block containing 32-bit zone numbers
type MapBlock []uint16            // block containing bitmaps (in 16-bit chunks)
type FullDataBlock []uint8        // block containing data (in bytes)
type PartialDataBlock []uint8     // block containing data (in bytes)

func (b InodeBlock) isBlockType()       {}
func (b DirectoryBlock) isBlockType()   {}
func (b IndirectBlock) isBlockType()    {}
func (b MapBlock) isBlockType()         {}
func (b FullDataBlock) isBlockType()    {}
func (b PartialDataBlock) isBlockType() {}

// Ensure each block type implements the Block interface
var _ Block = (InodeBlock)(nil)
var _ Block = (DirectoryBlock)(nil)
var _ Block = (IndirectBlock)(nil)
var _ Block = (MapBlock)(nil)
var _ Block = (FullDataBlock)(nil)
var _ Block = (PartialDataBlock)(nil)
