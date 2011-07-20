package minixfs

import "log"
import "os"
import "sync"

// Inode is the in-memory inode structure, containing the full disk_inode as
// an embedded member. These are cached by the InodeCache, but they are not
// re-used.
type Inode struct {
	// The following fields are static and will not be altered once an inode
	// is created.
	dev  int
	inum uint

	// The following fields will be altered, so access must be performed under
	// a mutex.
	_count uint
	_dirty bool
	_mount bool

	_disk *disk_inode

	// The mutexes for the volatile fields
	m_count *sync.RWMutex
	m_dirty *sync.RWMutex
	m_mount *sync.RWMutex

	m_disk *sync.RWMutex
}

func NewInode() *Inode {
	inode := new(Inode)
	inode.m_count = new(sync.RWMutex)
	inode.m_dirty = new(sync.RWMutex)
	inode.m_mount = new(sync.RWMutex)
	inode.m_disk = new(sync.RWMutex)
	return inode
}

func (inode *Inode) Dev() int {
	return inode.dev
}

func (inode *Inode) Count() uint {
	inode.m_count.RLock()
	defer inode.m_count.RUnlock()
	return inode._count
}

func (inode *Inode) IncCount() {
	inode.m_count.RLock()
	defer inode.m_count.RUnlock()
	inode._count++
}

func (inode *Inode) DecCount() {
	inode.m_count.RLock()
	defer inode.m_count.RUnlock()
	inode._count--
}

func (inode *Inode) SetCount(count uint) {
	inode.m_count.Lock()
	defer inode.m_count.Unlock()
	inode._count = count
}

func (inode *Inode) Dirty() bool {
	inode.m_dirty.RLock()
	defer inode.m_dirty.RUnlock()
	return inode._dirty
}

func (inode *Inode) SetDirty(dirty bool) {
	inode.m_dirty.Lock()
	defer inode.m_dirty.Unlock()
	inode._dirty = dirty
}

func (inode *Inode) Mount() bool {
	inode.m_mount.RLock()
	defer inode.m_mount.RUnlock()
	return inode._mount
}

func (inode *Inode) SetMount(mount bool) {
	inode.m_mount.Lock()
	defer inode.m_mount.Unlock()
	inode._mount = mount
}

// Setters/Getters for the on-disk portion of the inode, sharing a single
// mutex.
func (inode *Inode) Mode() uint16 {
	inode.m_disk.RLock()
	defer inode.m_disk.RUnlock()
	return inode._disk.Mode
}

func (inode *Inode) SetMode(mode uint16) {
	inode.m_disk.Lock()
	defer inode.m_disk.Unlock()
	inode._disk.Mode = mode
}

func (inode *Inode) Nlinks() uint16 {
	inode.m_disk.RLock()
	defer inode.m_disk.RUnlock()
	return inode._disk.Nlinks
}

func (inode *Inode) SetNlinks(nlinks uint16) {
	inode.m_disk.Lock()
	defer inode.m_disk.Unlock()
	inode._disk.Nlinks = nlinks
}

func (inode *Inode) IncNlinks() {
	inode.m_disk.Lock()
	defer inode.m_disk.Unlock()
	inode._disk.Nlinks++
}

func (inode *Inode) DecNlinks() {
	inode.m_disk.Lock()
	defer inode.m_disk.Unlock()
	inode._disk.Nlinks--
}

func (inode *Inode) Uid() int16 {
	inode.m_disk.RLock()
	defer inode.m_disk.RUnlock()
	return inode._disk.Uid
}

func (inode *Inode) SetUid(uid int16) {
	inode.m_disk.Lock()
	defer inode.m_disk.Unlock()
	inode._disk.Uid = uid
}

func (inode *Inode) Gid() uint16 {
	inode.m_disk.RLock()
	defer inode.m_disk.RUnlock()
	return inode._disk.Gid
}

func (inode *Inode) SetGid(gid uint16) {
	inode.m_disk.Lock()
	defer inode.m_disk.Unlock()
	inode._disk.Gid = gid
}

func (inode *Inode) Size() int32 {
	inode.m_disk.RLock()
	defer inode.m_disk.RUnlock()
	return inode._disk.Size
}

func (inode *Inode) SetSize(size int32) {
	inode.m_disk.Lock()
	defer inode.m_disk.Unlock()
	inode._disk.Size = size
}

// TODO: Implement time getters/setters

func (inode *Inode) Zone(num int) uint32 {
	inode.m_disk.RLock()
	defer inode.m_disk.RUnlock()
	return inode._disk.Zone[num]
}

func (inode *Inode) SetZone(num int, zone uint32) {
	inode.m_disk.Lock()
	defer inode.m_disk.Unlock()
	inode._disk.Zone[num] = zone
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

	inode.SetMode(mode)
	inode.SetNlinks(0)
	inode.SetUid(0) // TODO: Must get the current uid
	inode.SetGid(0) // TODO: Must get the current gid

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
	inode.SetSize(0)
	// TODO: Update ATIME, CTIME, MTIME
	inode.SetDirty(true)

	// Acquire the 'disk' lock, since we need to alter those elements
	inode.m_disk.Lock()
	inode._disk.Zone = *new([10]uint32)
	for i := 0; i < 10; i++ {
		inode._disk.Zone[i] = NO_ZONE
	}
	inode.m_disk.Unlock()
}

func (fs *FileSystem) dup_inode(inode *Inode) {
	inode.IncCount()
}

// The caller is no longer using this inode. If no one else is using it
// either write it back to the disk immediately. If it has no links,
// truncate it and return it to the pool of available inodes.
func (fs *FileSystem) put_inode(rip *Inode) {
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
