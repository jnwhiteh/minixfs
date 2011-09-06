package minixfs

import "log"
import "os"

// Inode is the in-memory inode structure, containing the full disk_inode as
// an embedded member. These are cached by the InodeCache, but they are not
// re-used.
type Inode struct {
	// The following fields are static and will not be altered once an inode
	// is created.
	dev  int
	inum uint

	// The following fields can and will be altered
	count uint
	dirty bool
	mount bool

	disk *disk_inode
	super *Superblock
}

func NewInode() *Inode {
	inode := new(Inode)
	return inode
}

func (inode *Inode) Dev() int {
	return inode.dev
}

func (inode *Inode) Count() uint {
	return inode.count
}

func (inode *Inode) IncCount() {
	inode.count++
}

func (inode *Inode) DecCount() {
	inode.count--
}

func (inode *Inode) SetCount(count uint) {
	inode.count = count
}

func (inode *Inode) Dirty() bool {
	return inode.dirty
}

func (inode *Inode) SetDirty(dirty bool) {
	inode.dirty = dirty
}

func (inode *Inode) Mount() bool {
	return inode.mount
}

func (inode *Inode) SetMount(mount bool) {
	inode.mount = mount
}

// Setters/Getters for the on-disk portion of the inode, sharing a single
// mutex.
func (inode *Inode) Mode() uint16 {
	return inode.disk.Mode
}

func (inode *Inode) SetMode(mode uint16) {
	inode.disk.Mode = mode
}

func (inode *Inode) Nlinks() uint16 {
	return inode.disk.Nlinks
}

func (inode *Inode) SetNlinks(nlinks uint16) {
	inode.disk.Nlinks = nlinks
}

func (inode *Inode) IncNlinks() {
	inode.disk.Nlinks++
}

func (inode *Inode) DecNlinks() {
	inode.disk.Nlinks--
}

func (inode *Inode) Uid() int16 {
	return inode.disk.Uid
}

func (inode *Inode) SetUid(uid int16) {
	inode.disk.Uid = uid
}

func (inode *Inode) Gid() uint16 {
	return inode.disk.Gid
}

func (inode *Inode) SetGid(gid uint16) {
	inode.disk.Gid = gid
}

func (inode *Inode) Size() int32 {
	return inode.disk.Size
}

func (inode *Inode) SetSize(size int32) {
	inode.disk.Size = size
}

// TODO: Implement time getters/setters

func (inode *Inode) Zone(num int) uint32 {
	return inode.disk.Zone[num]
}

func (inode *Inode) SetZone(num int, zone uint32) {
	inode.disk.Zone[num] = zone
}

// Accessors for the superblock portion
func (inode *Inode) Scale() uint {
	return inode.super.Log_zone_size
}

func (inode *Inode) BlockSize() int {
	return int(inode.super.Block_size)
}

// Utility functions

// GetType returns the type of an inode, extracting it from the mode
func (inode *Inode) GetType() uint16 {
	return inode.Mode() & I_TYPE
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
func (fs *fileSystem) get_inode(dev int, num uint) (*Inode, os.Error) {
	return fs.icache.GetInode(dev, num)
}

// Allocate a free inode on the given device and return a pointer to it.
func (fs *fileSystem) alloc_inode(dev int, mode uint16) *Inode {
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

	inode.SetMode(mode)
	inode.SetNlinks(0)
	inode.SetUid(0) // TODO: Must get the current uid
	inode.SetGid(0) // TODO: Must get the current gid

	fs.wipe_inode(inode)
	return inode
}

// Return an inode to the pool of free inodes
func (fs *fileSystem) free_inode(dev int, inumb uint) {
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

func (fs *fileSystem) wipe_inode(inode *Inode) {
	inode.SetSize(0)
	// TODO: Update ATIME, CTIME, MTIME
	inode.SetDirty(true)

	// Acquire the 'disk' lock, since we need to alter those elements
	inode.disk.Zone = *new([10]uint32)
	for i := 0; i < 10; i++ {
		inode.disk.Zone[i] = NO_ZONE
	}
}

func (fs *fileSystem) dup_inode(inode *Inode) {
	inode.IncCount()
}

// The caller is no longer using this inode. If no one else is using it
// either write it back to the disk immediately. If it has no links,
// truncate it and return it to the pool of available inodes.
func (fs *fileSystem) put_inode(rip *Inode) {
	if rip == nil {
		return
	}

	rip.DecCount()
	if rip.Count() == 0 { // means no one is using it now
		if rip.Nlinks() == 0 { // free the inode
			fs.truncate(rip) // return all the disk blocks
			rip.SetMode(I_NOT_ALLOC)
			rip.SetDirty(true)
			fs.free_inode(rip.dev, rip.inum)
		} else {
			// TODO: Handle the pipe case here
			// if rip.pipe == true {
			//   truncate(rip)
			// }
		}
		// rip.pipe = false
		if rip.Dirty() {
			fs.icache.WriteInode(rip)
		}
	}
}
