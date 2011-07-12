package minixfs

import "log"
import "os"

// In-memory inode structure, containing the full disk_inode as an embedded
// member.
type Inode struct {
	*disk_inode
	dev   int
	count uint
	inum  uint
	dirty bool
	mount bool
}

// GetType returns the type of an inode, extracting it from the mode
func (inode *Inode) GetType() uint16 {
	return inode.Mode & I_TYPE
}

// IsDirectory return true if the inode represents a directory on the file
// system
func (inode *Inode) IsDirectory() bool {
	return inode.GetType() == I_DIRECTORY
}

// IsRegular returns whether or not the inode represents a regular data file
// on the file system.
func (inode *Inode) IsRegular() bool {
	return inode.GetType() == I_REGULAR
}

// Retrieve an Inode from disk/cache given an Inode number. The 0th Inode
// is reserved and unallocatable, so we return an error when it is requested
// The root inode on the disk is ROOT_INODE_NUM, and should be located 64
// bytes into the first block following the bitmaps.
func (fs *FileSystem) get_inode(dev int, num uint) (*Inode, os.Error) {
	return fs.icache.GetInode(dev, num)
}

// Allocate a free inode on the given device and return a pointer to it.
func (fs *FileSystem) alloc_inode(dev int, mode uint16) *Inode {
	super := fs.supers[dev]

	// Acquire an inode from the bit map
	b := fs.alloc_bit(dev, IMAP, super.I_Search)
	if b == NO_BIT {
		log.Printf("Out of i-nodes on device")
		return nil
	}

	super.I_Search = b // next time start here

	// Try to acquire a slot in the inode table
	inode, err := fs.get_inode(dev, b)
	if err != nil {
		log.Printf("Failed to get inode: %d", b)
		return nil
	}

	inode.Mode = mode
	inode.Nlinks = 0
	inode.Uid = 0 // TODO: Must get the current uid
	inode.Gid = 0 // TODO: Must get the current gid

	fs.wipe_inode(inode)
	return inode
}

// Return an inode to the pool of free inodes
func (fs *FileSystem) free_inode(dev int, inumb uint) {
	sp := fs.supers[dev]
	if inumb <= 0 || inumb > sp.Ninodes {
		return
	}
	b := inumb
	fs.free_bit(dev, IMAP, b)

	if b < sp.I_Search {
		sp.I_Search = b
	}
}

func (fs *FileSystem) wipe_inode(inode *Inode) {
	inode.Size = 0
	// TODO: Update ATIME, CTIME, MTIME
	inode.dirty = true
	inode.Zone = *new([10]uint32)
	for i := 0; i < 10; i++ {
		inode.Zone[i] = NO_ZONE
	}
}

func (fs *FileSystem) dup_inode(inode *Inode) {
	inode.count++
}

// The caller is no longer using this inode. If no one else is using it
// either write it back to the disk immediately. If it has no links,
// truncate it and return it to the pool of available inodes.
func (fs *FileSystem) put_inode(rip *Inode) {
	if rip == nil {
		return
	}

	rip.count--
	if rip.count == 0 { // means no one is using it now
		if rip.Nlinks == 0 { // free the inode
			fs.truncate(rip) // return all the disk blocks
			rip.Mode = I_NOT_ALLOC
			rip.dirty = true
			fs.free_inode(rip.dev, rip.inum)
		} else {
			// TODO: Handle the pipe case here
			// if rip.pipe == true {
			//   truncate(rip)
			// }
		}
		// rip.pipe = false
		if rip.dirty {
			fs.icache.WriteInode(rip)
		}
	}
}
