package minixfs

import "log"
import "os"

type Inode struct {
	*disk_inode
	fs    *FileSystem
	count uint
	inum  uint
	dirty bool
}

func (inode *Inode) GetType() uint16 {
	return inode.Mode & I_TYPE
}

func (inode *Inode) IsDirectory() bool {
	return inode.GetType() == I_DIRECTORY
}

func (inode *Inode) IsRegular() bool {
	return inode.GetType() == I_REGULAR
}

func (inode *Inode) Inum() uint {
	return inode.inum
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
	bp := fs.GetBlock(int(block_num), INODE_BLOCK)
	inodeb := bp.block.(InodeBlock)

	// We have the full block, now get the correct inode entry
	inode_d := &inodeb[(num-1)%fs.super.inodes_per_block]
	inode := &Inode{
		disk_inode: inode_d,
		fs:         fs,
		count:      1,
		inum:       num,
		dirty:      false,
	}
	fs.inodes[num] = inode

	return inode, nil
}

// Allocate a free inode on the given FileSystem and return a pointer to it.
func (fs *FileSystem) AllocInode(mode uint16) *Inode {
	// Acquire an inode from the bit map
	b := fs.AllocBit(IMAP, fs.super.I_Search)
	if b == NO_BIT {
		log.Printf("Out of i-nodes on device")
		return nil
	}

	fs.super.I_Search = b

	// Try to acquire a slot in the inode table
	inode, err := fs.GetInode(b)
	if err != nil {
		log.Printf("Failed to get inode: %d", b)
		return nil
	}

	inode.Mode = mode
	inode.Nlinks = 0
	inode.Uid = 0 // TODO: Must get the current uid
	inode.Gid = 0 // TODO: Must get the current gid

	fs.WipeInode(inode)
	return inode
}

// Return an inode to the pool of free inodes
func (fs *FileSystem) FreeInode(inode *Inode) {
	fs.FreeBit(IMAP, inode.inum)
	if inode.inum < fs.super.I_Search {
		fs.super.I_Search = inode.inum
	}
}

func (fs *FileSystem) WipeInode(inode *Inode) {
	inode.Size = 0
	// TODO: Update ATIME, CTIME, MTIME
	inode.dirty = true
	inode.Zone = *new([10]uint32)
	for i := 0; i < 10; i++ {
		inode.Zone[i] = NO_ZONE
	}
}

func (fs *FileSystem) DupInode(inode *Inode) {
	inode.count++
}

// The caller is no longer using this inode. If no one else is using it
// either write it back to the disk immediately. If it has no links,
// truncate it and return it to the pool of available inodes.
func (fs *FileSystem) PutInode(rip *Inode) {
	if rip == nil {
		return
	}
	rip.count--
	if rip.count == 0 { // means no one is using it now
		if rip.Nlinks == 0 { // free the inode
			fs.Truncate(rip) // return all the disk blocks
			rip.Mode = I_NOT_ALLOC
			rip.dirty = true
			fs.FreeInode(rip)
		} else {
			// TODO: Handle the pipe case here
			// if rip.pipe == true {
			//   truncate(rip)
			// }
		}
		// rip.pipe = false
		if rip.dirty {
			// TODO: Implement RWInode, which will write the inode block back
			//fs.RWInode(rip, WRITING)
		}
	}
}
