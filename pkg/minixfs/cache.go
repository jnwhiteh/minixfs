package minixfs

import "os"

// Buffer (block) cache. To acquire a block, a routine calls fs.GetBlock()
// indicating which block it wants. The block is then regarded as "in use"
// and has its 'Count' field incremented.
//
// If a block is modified, the modifying routine must set Dirty to dirty
// to 'true' so the block will eventually be rewritten to the disk.
//
// This struct is embedded in every 'Block' type

type Buf struct {
	num   uint // the block number for this block
	dirty bool // clean (false) or dirty (true)
	count uint // the number of users of this cache block
}

func (fs *FileSystem) GetInodeBlock(num uint) (*InodeBlock, os.Error) {
	block := fs.NewInodeBlock()
	err := fs.GetBlock(num, block)
	return block, err
}

func (fs *FileSystem) GetDirectoryBlock(num uint) (*DirectoryBlock, os.Error) {
	block := fs.NewDirectoryBlock()
	err := fs.GetBlock(num, block)
	return block, err
}

func (fs *FileSystem) GetIndirectBlock(num uint) (*IndirectBlock, os.Error) {
	block := fs.NewIndirectBlock()
	err := fs.GetBlock(num, block)
	return block, err
}

func (fs *FileSystem) GetMapBlock(num uint) (*MapBlock, os.Error) {
	block := fs.NewMapBlock()
	err := fs.GetBlock(num, block)
	return block, err
}

func (fs *FileSystem) GetFullDataBlock(num uint) (*FullDataBlock, os.Error) {
	block := fs.NewFullDataBlock()
	err := fs.GetBlock(num, block)
	return block, err
}

func (fs *FileSystem) GetPartialDataBlock(num uint) (*PartialDataBlock, os.Error) {
	block := fs.NewPartialDataBlock()
	err := fs.GetBlock(num, block)
	return block, err
}

// TODO: Refactor this to use rw_block
func (fs *FileSystem) GetBlock(num uint, block Block) os.Error {
	if num <= 0 {
		return os.NewError("Invalid block requested")
	}

	var err os.Error
	pos := int64((num) * uint(fs.super.Block_size))

	// Do a type assertion and perform the actual I/O.
	if bp, ok := block.(*InodeBlock); ok {
		err = fs.dev.Read(bp.Data, pos)
		bp.buf.num = num
		bp.buf.dirty = false
	} else if bp, ok := block.(*DirectoryBlock); ok {
		err = fs.dev.Read(bp.Data, pos)
		bp.buf.num = num
		bp.buf.dirty = false
	} else if bp, ok := block.(*IndirectBlock); ok {
		err = fs.dev.Read(bp.Data, pos)
		bp.buf.num = num
		bp.buf.dirty = false
	} else if bp, ok := block.(*MapBlock); ok {
		err = fs.dev.Read(bp.Data, pos)
		bp.buf.num = num
		bp.buf.dirty = false
	} else if bp, ok := block.(*FullDataBlock); ok {
		err = fs.dev.Read(bp.Data, pos)
		bp.buf.num = num
		bp.buf.dirty = false
	} else if bp, ok := block.(*PartialDataBlock); ok {
		err = fs.dev.Read(bp.Data, pos)
		bp.buf.num = num
		bp.buf.dirty = false
	} else {
		err = os.NewError("Invalid block type")
	}

	if err != nil {
		return err
	}

	return nil
}

// Return a block to the list of available blocks.
func (fs *FileSystem) PutBlock(block Block, block_type int) os.Error {

	// Get the block number and dirty flag
	var num uint
	var dirty bool
	if bp, ok := block.(*InodeBlock); ok {
		num = bp.buf.num
		dirty = bp.buf.dirty
	} else if bp, ok := block.(*DirectoryBlock); ok {
		num = bp.buf.num
		dirty = bp.buf.dirty
	} else if bp, ok := block.(*IndirectBlock); ok {
		num = bp.buf.num
		dirty = bp.buf.dirty
	} else if bp, ok := block.(*MapBlock); ok {
		num = bp.buf.num
		dirty = bp.buf.dirty
	} else if bp, ok := block.(*FullDataBlock); ok {
		num = bp.buf.num
		dirty = bp.buf.dirty
	} else if bp, ok := block.(*PartialDataBlock); ok {
		num = bp.buf.num
		dirty = bp.buf.dirty
	}

	if dirty {
		var err os.Error
		pos := int64((num) * uint(fs.super.Block_size))

		// Do a type assertion and perform the actual I/O.
		if bp, ok := block.(*InodeBlock); ok {
			err = fs.dev.Write(bp.Data, pos)
		} else if bp, ok := block.(*DirectoryBlock); ok {
			err = fs.dev.Write(bp.Data, pos)
		} else if bp, ok := block.(*IndirectBlock); ok {
			err = fs.dev.Write(bp.Data, pos)
		} else if bp, ok := block.(*MapBlock); ok {
			err = fs.dev.Write(bp.Data, pos)
		} else if bp, ok := block.(*FullDataBlock); ok {
			err = fs.dev.Write(bp.Data, pos)
		} else if bp, ok := block.(*PartialDataBlock); ok {
			err = fs.dev.Write(bp.Data, pos)
		}

		if err != nil {
			return err
		}
	}
	return nil
}

// Return a zone
func (fs *FileSystem) FreeZone(numb uint) {
	if numb < fs.super.Firstdatazone_old || numb >= fs.super.Nzones {
		return
	}
	bit := numb - fs.super.Firstdatazone_old - 1
	fs.FreeBit(ZMAP, bit)
	if bit < fs.super.I_Search {
		fs.super.I_Search = bit
	}
}
