package minixfs

import "log"
import "os"

// Inode is the in-memory inode structure, containing the full disk_inode as
// an embedded member. These are cached by the InodeCache, but they are not
// re-used.
type CacheInode struct {
	// The following fields are static and will not be altered once an inode
	// is created.
	dev  int
	inum uint

	// The following fields can and will be altered
	count uint
	dirty bool
	mount bool

	disk  *disk_inode
	super *Superblock // the superblock for this filesystem
}

func NewInode() *CacheInode {
	inode := new(CacheInode)
	return inode
}

func (inode *CacheInode) Dev() int {
	return inode.dev
}

func (inode *CacheInode) Count() uint {
	return inode.count
}

func (inode *CacheInode) IncCount() {
	inode.count++
}

func (inode *CacheInode) DecCount() {
	inode.count--
}

func (inode *CacheInode) SetCount(count uint) {
	inode.count = count
}

func (inode *CacheInode) Dirty() bool {
	return inode.dirty
}

func (inode *CacheInode) SetDirty(dirty bool) {
	inode.dirty = dirty
}

func (inode *CacheInode) Mount() bool {
	return inode.mount
}

func (inode *CacheInode) SetMount(mount bool) {
	inode.mount = mount
}

// Setters/Getters for the on-disk portion of the inode, sharing a single
// mutex.
func (inode *CacheInode) Mode() uint16 {
	return inode.disk.Mode
}

func (inode *CacheInode) SetMode(mode uint16) {
	inode.disk.Mode = mode
}

func (inode *CacheInode) Nlinks() uint16 {
	return inode.disk.Nlinks
}

func (inode *CacheInode) SetNlinks(nlinks uint16) {
	inode.disk.Nlinks = nlinks
}

func (inode *CacheInode) IncNlinks() {
	inode.disk.Nlinks++
}

func (inode *CacheInode) DecNlinks() {
	inode.disk.Nlinks--
}

func (inode *CacheInode) Uid() int16 {
	return inode.disk.Uid
}

func (inode *CacheInode) SetUid(uid int16) {
	inode.disk.Uid = uid
}

func (inode *CacheInode) Gid() uint16 {
	return inode.disk.Gid
}

func (inode *CacheInode) SetGid(gid uint16) {
	inode.disk.Gid = gid
}

func (inode *CacheInode) Size() int32 {
	return inode.disk.Size
}

func (inode *CacheInode) SetSize(size int32) {
	inode.disk.Size = size
}

// TODO: Implement time getters/setters

func (inode *CacheInode) Zone(num int) uint32 {
	return inode.disk.Zone[num]
}

func (inode *CacheInode) SetZone(num int, zone uint32) {
	inode.disk.Zone[num] = zone
}

// Accessors for the superblock portion
func (inode *CacheInode) Scale() uint {
	return inode.super.Log_zone_size
}

func (inode *CacheInode) BlockSize() int {
	return int(inode.super.Block_size)
}

func (inode *CacheInode) Firstdatazone() int {
	return int(inode.super.Firstdatazone)
}

func (inode *CacheInode) Zones() int {
	return int(inode.super.Zones)
}

// Utility functions

// GetType returns the type of an inode, extracting it from the mode
func (inode *CacheInode) GetType() uint16 {
	return inode.Mode() & I_TYPE
}

// IsDirectory return true if the inode represents a directory on the file
// system
func (inode *CacheInode) IsDirectory() bool {
	return inode.GetType() == I_DIRECTORY
}

// IsRegular returns whether or not the inode represents a regular data file
// on the file system.
func (inode *CacheInode) IsRegular() bool {
	return inode.GetType() == I_REGULAR
}

// Retrieve an Inode from disk/cache given an Inode number. The 0th Inode
// is reserved and unallocatable, so we return an error when it is requested
// The root inode on the disk is ROOT_INODE_NUM, and should be located 64
// bytes into the first block following the bitmaps.
func (fs *fileSystem) get_inode(dev int, num uint) (*CacheInode, os.Error) {
	return fs.icache.GetInode(dev, num)
}

func (fs *fileSystem) wipe_inode(inode *CacheInode) {
	inode.SetSize(0)
	// TODO: Update ATIME, CTIME, MTIME
	inode.SetDirty(true)

	// Acquire the 'disk' lock, since we need to alter those elements
	inode.disk.Zone = *new([10]uint32)
	for i := 0; i < 10; i++ {
		inode.disk.Zone[i] = NO_ZONE
	}
}

func (fs *fileSystem) dup_inode(inode *CacheInode) {
	inode.IncCount()
}

// The caller is no longer using this inode. If no one else is using it
// either write it back to the disk immediately. If it has no links,
// truncate it and return it to the pool of available inodes.
func (fs *fileSystem) put_inode(rip *CacheInode) {
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
