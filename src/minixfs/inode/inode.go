package inode

import (
	. "minixfs/common"
	"sync"
)

type inodeLockMode int

type cacheInode struct {
	*Disk_Inode   // the inode as stored on disk
	*sync.RWMutex // this lock must be acquired for any inode operation

	bcache  BlockCache // the block cache
	icache  InodeCache // the inode cache
	bitmap  Bitmap     // the bitmap for the inode's device (for allocation)
	devinfo DeviceInfo // the device information for the inode's device

	devnum int // the device number
	inum   int // the inode number

	count int  // the number of clients of this inode
	dirty bool // whether or not the inode has been changed
	mount bool // whether or not this inode is used as a mount point

	locked bool // whether or not the inode is/was in a locked state
}

//////////////////////////////////////////////////////////////////////////////
// Static methods
//
// These methods are always available without locking
//////////////////////////////////////////////////////////////////////////////

func (rip *cacheInode) Devnum() int {
	return rip.devnum
}

func (rip *cacheInode) Inum() int {
	return rip.inum
}

func (rip *cacheInode) Type() int {
	return int(rip.Mode & I_TYPE)
}

func (rip *cacheInode) IsRegular() bool {
	return rip.Mode&I_TYPE == I_REGULAR
}

func (rip *cacheInode) IsDirectory() bool {
	return rip.Mode&I_TYPE == I_DIRECTORY
}

//////////////////////////////////////////////////////////////////////////////
// Read methods
//
// These methods are get operations for the parameters of the inode
//////////////////////////////////////////////////////////////////////////////

func (rip *cacheInode) GetMode() int {
	return int(rip.Mode)
}

func (rip *cacheInode) Links() int {
	return int(rip.Nlinks)
}

func (rip *cacheInode) IsDirty() bool {
	return rip.dirty
}

func (rip *cacheInode) MountPoint() bool {
	return rip.mount
}

func (rip *cacheInode) GetSize() int {
	return int(rip.Disk_Inode.Size)
}

func (rip *cacheInode) GetZone(znum int) uint32 {
	return rip.Disk_Inode.Zone[znum]
}
//////////////////////////////////////////////////////////////////////////////
// Write methods
//
// These methods may/do change the fields of an inode, so they must be run
// under the 'write' portion of a RWMutex. This is done using the type system
// to ensure they are only invoked on a LockedInode.
//////////////////////////////////////////////////////////////////////////////

func (rip *cacheInode) IncLinks() {
	rip.Nlinks++
}

func (rip *cacheInode) DecLinks() {
	rip.Nlinks--
}

func (rip *cacheInode) SetDirty(dirty bool) {
	rip.dirty = dirty
}

func (rip *cacheInode) SetMountPoint(mount bool) {
	rip.mount = mount
}

func (rip *cacheInode) Count() int {
	return rip.count
}

func (rip *cacheInode) SetMode(mode uint16) {
	rip.Mode = mode
}

func (rip *cacheInode) SetZone(znum int, zone uint32) {
	rip.Zone[znum] = zone
}
