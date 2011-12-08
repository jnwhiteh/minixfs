package fs

import (
	. "../../minixfs/common/_obj/minixfs/common"
	"../bcache/_obj/minixfs/bcache"
	"../device/_obj/minixfs/device"
	"../icache/_obj/minixfs/icache"
	"../super/_obj/minixfs/super"
	"encoding/binary"
	"log"
	"os"
	"sync"
)

// FileSystem encapsulates a MINIX file system.
type FileSystem struct {
	devs    []RandDevice // the block devices that comprise the file system
	devinfo []DeviceInfo // the geometry/params for the given device
	supers  []Superblock // the superblocks for the given devices

	bcache BlockCache // the block cache (shared across all devices)
	icache InodeCache // the inode cache (shared across all devices)

	filps []*filp    // the filep (file position) table
	procs []*Process // an array of processes that have been spawned

	// Locks protecting the above slices
	m struct {
		device *sync.RWMutex
		filp   *sync.RWMutex
		proc   *sync.RWMutex
	}
}

// Create a new FileSystem from a given file on the filesystem
func OpenFileSystemFile(filename string) (*FileSystem, os.Error) {
	dev, err := device.NewFileDevice(filename, binary.LittleEndian)

	if err != nil {
		return nil, err
	}

	return NewFileSystem(dev)
}

// Create a new FileSystem from a given file on the filesystem
func NewFileSystem(dev RandDevice) (*FileSystem, os.Error) {
	fs := new(FileSystem)

	fs.devs = make([]RandDevice, NR_SUPERS)
	fs.devinfo = make([]DeviceInfo, NR_SUPERS)
	fs.supers = make([]Superblock, NR_SUPERS)

	sblock, devinfo, err := super.ReadSuperblock(dev)
	if err != nil {
		return nil, err
	}

	fs.devs[ROOT_DEVICE] = dev
	fs.devinfo[ROOT_DEVICE] = devinfo
	fs.supers[ROOT_DEVICE] = sblock

	fs.bcache = bcache.NewLRUCache(NR_SUPERS, NR_BUFS, NR_BUF_HASH)
	fs.icache = icache.NewInodeCache(fs.bcache, NR_SUPERS, NR_INODES)

	fs.filps = make([]*filp, NR_INODES)
	fs.procs = make([]*Process, NR_PROCS)

	if err := fs.bcache.MountDevice(ROOT_DEVICE, dev, devinfo); err != nil {
		log.Printf("Could not mount root device: %s", err)
		return nil, err
	}
	fs.icache.MountDevice(ROOT_DEVICE, sblock, devinfo)

	// Fetch the root inode
	rip, err := fs.icache.GetInode(ROOT_DEVICE, ROOT_INODE)
	if err != nil {
		log.Printf("Failed to fetch root inode: %s", err)
		return nil, err
	}

	// Create the root process
	fs.procs[ROOT_PROCESS] = &Process{ROOT_PROCESS, 022, rip, rip,
		make([]*filp, OPEN_MAX),
	}

	fs.m.device = new(sync.RWMutex)
	fs.m.filp = new(sync.RWMutex)
	fs.m.proc = new(sync.RWMutex)

	return fs, nil
}

//var _ fileSystem = FileSystem{}
