package fs

import (
	"fmt"
	"github.com/jnwhiteh/minixfs/common"
	"log"
)

type dirop int

const (
	LOOKUP   dirop = iota // search for 'path' and return inode # in 'inum'
	ENTER                 // add 'path' to the directory listing with inode # 'inum'
	DELETE                // remove 'path' from the directory listing
	IS_EMPTY              // return OK if only . and .. are in the dir, else ENOTEMPTY
)

func search_dir(dirp *common.Inode, path string, inum *int, op dirop) error {
	// TODO: Check permissions (see minix source)
	devinfo := dirp.Devinfo
	blocksize := devinfo.Blocksize

	// step through the directory on block at a time
	var bp *common.CacheBlock
	var dp *common.Disk_dirent
	old_slots := int(dirp.Size / common.DIR_ENTRY_SIZE)
	new_slots := 0
	e_hit := false
	match := false
	extended := false

	for pos := 0; pos < int(dirp.Size); pos += blocksize {
		b := common.ReadMap(dirp, pos, dirp.Bcache) // get block number
		if dirp.Bcache == nil {
			panic(fmt.Sprintf("No block cache: %q", dirp))
		}
		bp = dirp.Bcache.GetBlock(devinfo.Devnum, b, common.DIRECTORY_BLOCK, common.NORMAL)
		if bp == nil {
			panic("get_block returned NO_BLOCK")
		}

		// Search the directory block
		dirarr := bp.Block.(common.DirectoryBlock)
		for i := 0; i < len(dirarr); i++ {
			dp = &dirarr[i]
			new_slots++
			if new_slots > old_slots { // not found, but room left
				if op == ENTER {
					e_hit = true
				}
				break
			}

			// Match occurs if string found
			if op != ENTER && dp.Inum != 0 {
				if op == IS_EMPTY {
					// If this succeeds, dir is not empty
					if !dp.HasName(".") && !dp.HasName("..") {
						log.Printf("Found entry: %s (%d)", dp.String(), dp.Inum)
						match = true
					}
				} else {
					if dp.HasName(path) {
						match = true
					}
				}
			}

			if match {
				var r error = nil
				// LOOK_UP or DELETE found what it wanted
				if op == IS_EMPTY {
					r = common.ENOTEMPTY
				} else if op == DELETE {
					// TODO: Save inode for recovery
					dp.Inum = 0 // erase entry
					bp.Dirty = true
					dirp.Dirty = true
				} else {
					*inum = int(dp.Inum)
				}
				dirp.Bcache.PutBlock(bp, common.DIRECTORY_BLOCK)
				return r
			}

			// Check for free slot for the benefit of ENTER
			if op == ENTER && dp.Inum == 0 {
				e_hit = true // we found a free slot
				break
			}
		}

		// The whole block has been searched or ENTER has a free slot
		if e_hit { // e_hit set if ENTER can be performed now
			break
		}
		dirp.Bcache.PutBlock(bp, common.DIRECTORY_BLOCK) // otherwise continue searching dir
	}

	// The whole directory has now been searched
	if op != ENTER {
		if op == IS_EMPTY {
			return nil
		} else {
			return common.ENOENT
		}
	}

	// This call is for ENTER. If no free slot has been found so far, try to
	// extend directory.
	if !e_hit { // directory is full and no room left in last block
		new_slots++ // increase directory size by 1 entry
		// TODO: Does this rely on overflow? Does it work?
		if new_slots == 0 { // dir size limited by slot count (overflow?)
			return common.EFBIG
		}
		var err error
		bp, err = common.NewBlock(dirp, int(dirp.Size), common.DIRECTORY_BLOCK, dirp.Bcache)
		if err != nil {
			return err
		}
		dirarr := bp.Block.(common.DirectoryBlock)
		dp = &dirarr[0]
		extended = true
	}

	// 'bp' now points to a directory block with space. 'dp' points to a slot
	// in that block.

	// Set the name of this directory entry
	pathb := []byte(path)
	if len(pathb) < common.NAME_MAX {
		dp.Name[len(pathb)] = 0
	}
	for i := 0; i < common.NAME_MAX && i < len(pathb); i++ {
		dp.Name[i] = pathb[i]
	}
	dp.Inum = uint32(*inum)
	bp.Dirty = true

	dirp.Bcache.PutBlock(bp, common.DIRECTORY_BLOCK)
	// TODO: update times
	dirp.Dirty = true
	if new_slots > old_slots {
		dirp.Size = (int32(new_slots * common.DIR_ENTRY_SIZE))
		// Send the change to disk if the directory is extended
		if extended {
			// TODO: Write this inode out to the block cache
			dirp.Icache.FlushInode(dirp)
		}
	}
	return nil
}
