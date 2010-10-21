package minixfs

import "rand"
import "testing"
import "time"

func GetRandomBitmap(num_bits int) (Bitmap, []uint8) {
	rand.Seed(time.Nanoseconds())

	num_ints := num_bits / 8
	arr := make([]uint8, num_ints)
	for idx := 0; idx < num_ints; idx++ {
		arr[idx] = uint8(rand.Int())
	}
	return Bitmap(arr), arr
}

func TestGetBit(test *testing.T) {
	bmap, arr := GetRandomBitmap(32768)

	// Intel is little-endian, so the first bit we test will be the least significant
	// bit, i.e bit 0 is the first bit of the first uint8
	for i, num := range arr {
		for mask := uint8(1); mask <= 8; mask++ {
			// Calculate the 'bit number'
			bnum := uint(i*8) + uint(mask-1)
			bit := num&mask > 0
			result, err := bmap.GetBit(bnum)
			if err != nil {
				test.Errorf("Error occurred: %s", err)
			}
			if result != bit {
				test.Errorf("Test failed for bit: %d, ival: %d", bnum, num)
			}
		}
	}
}

func TestSetBit(test *testing.T) {
	bmap, _ := GetRandomBitmap(32768)
	for i := uint(0); i < bmap.GetNumBits(); i++ {
		err := bmap.SetBit(i)
		if err != nil {
			test.Errorf("Failed to set bit: %s", err)
		}
	}

	for i := uint(0); i < bmap.GetNumBits(); i++ {
		result, err := bmap.GetBit(i)
		if err != nil {
			test.Errorf("Failed to get bit: %s", err)
		}

		if !result {
			test.Errorf("Bit %d was not properly set: %q", i, result)
		}
	}
}

func TestClrBit(test *testing.T) {
	bmap, _ := GetRandomBitmap(32768)
	for i := uint(0); i < bmap.GetNumBits(); i++ {
		err := bmap.ClrBit(i)
		if err != nil {
			test.Errorf("Failed to clear bit: %s", err)
		}
	}

	for i := uint(0); i < bmap.GetNumBits(); i++ {
		result, err := bmap.GetBit(i)
		if err != nil {
			test.Errorf("Failed to get bit: %s", err)
		}

		if result {
			test.Errorf("Bit %d was not properly cleared: %q", i, result)
		}
	}
}
