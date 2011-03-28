package minixfs

import "encoding/binary"
import "log"
import "os"

// This type encapsulates a minix file system, including the shared data
// structures associated with the file system. It abstracts away from the
// file system residing on disk.

type FileSystem struct {
	file   *os.File        // the actual file backing the file system
	super  *Superblock     // the superblock for the associated file system
	inodes map[uint]*Inode // a map containing the inodes for the open files

	Magic         uint // magic number to recognize super-blocks
	Block_size    uint // block size in bytes
	Log_zone_size uint // log2 of blocks/zone
}

type Directory struct {
	Inum uint32
	Name [60]byte
}

// Create a new FileSystem from a given file on the filesystem
func OpenFileSystemFile(filename string) (*FileSystem, os.Error) {
	var fs *FileSystem = new(FileSystem)

	// open the file, but do not close it
	file, err := os.Open(filename, os.O_RDWR, 0)

	if err != nil {
		return nil, err
	}

	super, err := ReadSuperblock(file)
	if err != nil {
		return nil, err
	}

	fs.file = file
	fs.super = super
	fs.inodes = make(map[uint]*Inode)

	fs.Magic = super.Magic
	fs.Block_size = super.Block_size
	fs.Log_zone_size = super.Log_zone_size

	return fs, nil
}

// Retrieve an Inode from disk/cache given an Inode number. The 0th Inode
// is reserved and unallocatable, so we return an error when it is requested
// The root inode on the disk is ROOT_INODE_NUM, and should be located 64
// bytes into the first block following the bitmaps.

func (fs *FileSystem) GetInode(num uint) (*Inode, os.Error) {
	if num == 0 {
		return nil, os.NewError("Invalid inode number")
	}

	// Check and see if the inode is already loaded in memory
	if inode, ok := fs.inodes[num]; ok {
		inode.count++
		return inode, nil
	}

	if len(fs.inodes) >= NR_INODES {
		return nil, os.NewError("Too many open inodes")
	}

	// For a 4096 block size, inodes 0-63 reside in the first block
	block_offset := fs.super.Imap_blocks + fs.super.Zmap_blocks + 2
	block_num := ((num - 1) / fs.super.inodes_per_block) + uint(block_offset)

	// Load the inode from the disk and create in-memory version of it
	inode_block := make([]disk_inode, fs.super.inodes_per_block)

	err := fs.GetBlock(block_num, inode_block)
	if err != nil {
		return nil, err
	}

	// We have the full block, now get the correct inode entry
	inode_d := &inode_block[(num-1)%fs.super.inodes_per_block]
	inode := &Inode{inode_d, fs, 1, num}

	return inode, nil
}

func (fs *FileSystem) GetDataBlockFromZone(num uint) uint {
	// Move past the boot block, superblock and bitmats
	offset := uint(2 + fs.super.Imap_blocks + fs.super.Zmap_blocks)
	offset = offset + (uint(fs.super.Ninodes) / fs.super.inodes_per_block)
	return offset + num
}

func (fs *FileSystem) GetBlock(num uint, block interface{}) os.Error {
	if num <= 1 {
		panic("TODO: Fix this")
	}

	// Adjust the file position according to two static blocks at start
	pos := int64((num) * uint(fs.super.Block_size))
	//log.Printf("seeking to pos: %d", pos)
	newPos, err := fs.file.Seek(pos, 0)
	if err != nil || pos != newPos {
		return err
	}

	err = binary.Read(fs.file, binary.LittleEndian, block)
	if err != nil {
		return err
	}

	return nil
}

func (fs *FileSystem) PutBlock(num uint, block interface{}) os.Error {
	if num <= 1 {
		panic("TODO: Fix this")
	}

	pos := int64((num) * uint(fs.super.Block_size))
	newPos, err := fs.file.Seek(pos, 0)
	if err != nil || pos != newPos {
		return err
	}

	err = binary.Write(fs.file, binary.LittleEndian, block)
	if err != nil {
		return err
	}

	return nil
}

// Given an inode and a position within the corresponding file, locate the
// block (not zone) number in which that position is to be found and return
func (fs *FileSystem) GetFileBlock(inode *Inode, position uint) uint {
	scale := fs.super.Log_zone_size                        // for block-zone conversion
	block_pos := position / fs.super.Block_size            // relative block # in file
	zone := block_pos >> scale                             // position's zone
	boff := block_pos - (zone << scale)                    // relative block in zone
	dzones := uint(V2_NR_DZONES)                           // number of direct zones
	nr_indirects := fs.super.Block_size / V2_ZONE_NUM_SIZE // number of indirect zones

	// Is the position to be found in the inode itself?
	if zone < dzones {
		z := uint(inode.Zone[zone])
		if z == NO_ZONE {
			return NO_BLOCK
		}
		b := (z << scale) + boff
		return b
	}

	// It is not in the inode, so must be single or double indirect
	var z uint
	var excess uint = zone - dzones

	if excess < nr_indirects {
		// 'position' can be located via the single indirect block
		z = uint(inode.Zone[dzones])
	} else {
		// 'position' can be located via the double indirect block
		z = uint(inode.Zone[dzones+1])
		if z == NO_ZONE {
			return NO_BLOCK
		}
		excess = excess - nr_indirects // single indirect doesn't count
		b := z << scale
		dindb := make([]uint32, fs.Block_size/4) // number of pointers in indirect block
		err := fs.GetBlock(uint(b), dindb)
		if err != nil {
			log.Printf("Could not fetch doubly-indirect block: %d - %s", b, err)
		}
		index := excess / nr_indirects
		z = uint(dindb[index])
		excess = excess % nr_indirects
	}

	// 'z' is zone num for single indirect block; 'excess' is index into it
	if z == NO_ZONE {
		return NO_BLOCK
	}

	b := z << scale                         // b is block number for single indirect
	indb := make([]uint32, fs.Block_size/4) // number of pointers in indirect block
	err := fs.GetBlock(uint(b), indb)
	if err != nil {
		log.Printf("Could not fetch indirect block: %d - %s", b, err)
		return NO_BLOCK
	}

	log.Printf("Getting position %d, have excess: %d", position, excess)
	z = uint(indb[excess])
	if z == NO_ZONE {
		return NO_BLOCK
	}
	b = (z << scale) + boff
	return b
}
