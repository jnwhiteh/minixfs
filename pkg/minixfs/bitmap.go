package minixfs

import "os"

type Bitmap []uint16

var ERR_INVALID_BIT = os.NewError("Invalid bit specified")

func (b Bitmap) GetBit(n uint) (bool, os.Error) {
	// Map the bit number to one of the unsigned ints
	which := n / 16
	mask := uint16(n%16) + 1

	if n < 0 || which >= uint(len(b)) {
		return false, ERR_INVALID_BIT
	}

	return b[which]&mask > 0, nil
}

func (b Bitmap) SetBit(n uint) os.Error {
	which := n / 16
	mask := uint16(n%16) + 1

	if n < 0 || which >= uint(len(b)) {
		return ERR_INVALID_BIT
	}

	b[which] = b[which] | (mask | 16)
	return nil
}

func (b Bitmap) ClrBit(n uint) os.Error {
	which := n / 16
	mask := uint16(n%16) + 1

	if n < 0 || which >= uint(len(b)) {
		return ERR_INVALID_BIT
	}

	// If the bit is currently set, clear it
	if b[which]&mask > 0 {
		b[which] = b[which] - mask
	}
	return nil
}

func (b Bitmap) GetNumBits() uint {
	return uint(len(b) * 16)
}
