package modules

import (
	"testing"

	"gitlab.com/NebulousLabs/fastrand"
)

// TestSialinkManualExamples checks a pile of manual examples using table driven
// tests.
func TestSialinkManualExamples(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	var sialinkExamples = []struct {
		offset         uint64
		length         uint64
		expectedLength uint64
	}{
		{0, 0, 4096},
		{0, 1, 4096},
		{0, 4095, 4096},
		{0, 4096, 4096},
		{0, 4097, 8192},
		{4096 * 45, 0, 4096},
		{0, 10e3, 4096 * 3},
		{0, 33e3, 4096 * 9},
		{0, 39e3, 4096 * 10},
		{8192 * 350, 39e3, 4096 * 10},
		{0, 71 * 1024, 72 * 1024},
		{0, (32 * 1024) - 1, 32 * 1024},
		{0, 32 * 1024, 32 * 1024},
		{0, (32 * 1024) + 1, 36 * 1024},
		{0, (64 * 1024) - 1, 64 * 1024},
		{8 * 1024, (64 * 1024) - 1, 64 * 1024},
		{16 * 1024, (64 * 1024) - 1, 64 * 1024},
		{0, (64 * 1024), 64 * 1024},
		{24 * 1024, (64 * 1024), 64 * 1024},
		{56 * 1024, (64 * 1024), 64 * 1024},
		{0, (64 * 1024) + 1, 72 * 1024},
		{16 * 1024, (64 * 1024) - 1, 64 * 1024},
		{48 * 1024, (64 * 1024) - 1, 64 * 1024},
		{16 * 1024, (64 * 1024), 64 * 1024},
		{48 * 1024, (64 * 1024), 64 * 1024},
		{16 * 1024, (64 * 1024) + 1, 72 * 1024},
		{48 * 1024, (64 * 1024) + 1, 72 * 1024},
		{16 * 1024, (72 * 1024) - 1, 72 * 1024},
		{48 * 1024, (72 * 1024) - 1, 72 * 1024},
		{16 * 1024, (72 * 1024), 72 * 1024},
		{48 * 1024, (72 * 1024), 72 * 1024},
		{16 * 1024, (72 * 1024) + 1, 80 * 1024},
		{48 * 1024, (72 * 1024) + 1, 80 * 1024},
		{192 * 1024, (288 * 1024) - 1, 288 * 1024},
	}

	// Try each example.
	for i, example := range sialinkExamples {
		var ld LinkData
		err := ld.SetOffsetAndLen(example.offset, example.length)
		if err != nil {
			t.Error(err)
		}
		offset, length := ld.OffsetAndLen()
		if offset != example.offset {
			t.Error("bad offset:", offset, example.offset, i)
		}
		if length != example.expectedLength {
			t.Error("bad length:", length, example.length, i)
		}
		if ld.Version() != 1 {
			t.Error("bad version:", ld.Version())
		}
	}
}

// TestSialink checks that the linkformat is correctly encoding to and decoding
// from a string.
func TestSialink(t *testing.T) {
	// Create a linkdata struct that is all 0's, check that the resulting
	// sialink is 52 bytes, and check that the struct encodes and decodes
	// without problems.
	var ldMin LinkData
	sialink := ldMin.Sialink()
	if len(sialink) != 52 {
		t.Error("sialink is not 52 bytes")
	}
	var ldMinDecoded LinkData
	err := ldMinDecoded.LoadSialink(sialink)
	if err != nil {
		t.Fatal(err)
	}
	if ldMinDecoded != ldMin {
		t.Error("encoding and decoding is not symmetric")
	}

	// Create a linkdata struct that is all 1's, check that the resulting
	// sialink is 52 bytes, and check that the struct encodes and decodes
	// without problems.
	ldMax := LinkData{
		bitfield: 65535,
	}
	ldMax.bitfield -= 7 // set the final three bits to 0 to make this a valid sialink.
	for i := 0; i < len(ldMax.merkleRoot); i++ {
		ldMax.merkleRoot[i] = 255
	}
	sialink = ldMax.Sialink()
	if len(sialink) != 52 {
		t.Error("sialink is not 52 bytes")
	}
	var ldMaxDecoded LinkData
	err = ldMaxDecoded.LoadSialink(sialink)
	if err != nil {
		t.Fatal(err)
	}
	if ldMaxDecoded != ldMax {
		t.Error("encoding and decoding is not symmetric")
	}

	// Try loading an arbitrary string that is too small.
	var ld LinkData
	var arb string
	for i := 0; i < encodedLinkDataSize-1; i++ {
		arb = arb + "a"
	}
	err = ld.LoadString(arb)
	if err == nil {
		t.Error("expecting error when loading string that is too small")
	}
	// Try loading a siafile that's just arbitrary/meaningless data.
	arb = arb + "a"
	err = ld.LoadString(arb)
	if err == nil {
		t.Error("arbitrary string should not decode")
	}
	// Try loading a siafile that's too large.
	long := arb + "a"
	err = ld.LoadString(long)
	if err == nil {
		t.Error("expecting error when loading string that is too large")
	}
	// Try loading a blank siafile.
	blank := ""
	err = ld.LoadString(blank)
	if err == nil {
		t.Error("expecting an error when loading a blank sialink")
	}

	// Try giving a sialink extra params and loading that.
	ldStr := ld.String()
	params := ldStr + "&fdsafdsafdsa"
	err = ld.LoadString(params)
	if err != nil {
		t.Error("should be no issues loading a sialink with params")
	}
	// Add more ampersands
	params = params + "&fffffdsafdsafdsa"
	err = ld.LoadString(params)
	if err != nil {
		t.Error("should be no issues loading a sialink with params")
	}

	// Some longer fuzzing sorts of tests below, skip for short tests.
	if testing.Short() {
		t.SkipNow()
	}
}

// TestSialinkAutoExamples performs a brute force test over lots of values for
// the sialink bitfield to ensure correctness.
func TestSialinkAutoExamples(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// Helper function to try some values.
	tryValues := func(offset, length, expectedLength uint64) {
		var ld LinkData
		err := ld.SetOffsetAndLen(offset, length)
		if err != nil {
			t.Error(err)
		}
		offsetOut, lengthOut := ld.OffsetAndLen()
		if offset != offsetOut {
			t.Error("bad offset:", offset, length, expectedLength, offsetOut)
		}
		if expectedLength != lengthOut {
			t.Error("bad length:", offset, length, expectedLength, lengthOut)
		}

		// Encode the sialink and then decode the sialink. There should be no
		// errors in doing so, and the result should equal the initial.
		sl := ld.Sialink()
		var ldDecode LinkData
		err = ldDecode.LoadSialink(sl)
		if err != nil {
			t.Error(err)
		}
		if ldDecode != ld {
			t.Log(ld)
			t.Error("linkdata does not maintain its fields when encoded and decoded")
		}
	}

	// Check every length in the first row. The first row must be offset by 4
	// kib.
	for i := uint64(0); i < 8; i++ {
		// Check every possible offset for each length.
		for j := uint64(0); j < 1024-i; j++ {
			// Try the edge cases. One byte into the length, one byte before the
			// end of the length, the very end of the length.
			shift := uint64(0)
			offsetAlign := uint64(4096)
			lengthAlign := uint64(4096)
			tryValues(offsetAlign*j, shift+((lengthAlign*i)+1), shift+(lengthAlign*(i+1)))
			tryValues(offsetAlign*j, shift+((lengthAlign*(i+1))-1), shift+(lengthAlign*(i+1)))
			tryValues(offsetAlign*j, shift+(lengthAlign*(i+1)), shift+(lengthAlign*(i+1)))

			// Try some random values.
			for k := uint64(0); k < 5; k++ {
				rand := uint64(fastrand.Intn(int(lengthAlign)))
				rand++                            // move range from [0, lengthAlign) to [1, lengthAlign].
				rand += shift + (lengthAlign * i) // Move range into the range being tested.
				tryValues(offsetAlign*j, rand, shift+(lengthAlign*(i+1)))
			}
		}
	}

	// The first row is a special case, a general loop can be used for the
	// remaining 7 rows.
	for r := uint64(1); r < 7; r++ {
		// Check every length in the second row.
		for i := uint64(0); i < 8; i++ {
			// Check every possible offset for each length.
			offsets := uint64(1024 >> r)
			for j := uint64(0); j < offsets-4-(i/2); j++ {
				// Try the edge cases. One byte into the length, one byte before the
				// end of the length, the very end of the length.
				shift := uint64(1 << (14 + r))
				offsetAlign := uint64(1 << (12 + r))
				lengthAlign := uint64(1 << (11 + r))
				tryValues(offsetAlign*j, shift+((lengthAlign*i)+1), shift+(lengthAlign*(i+1)))
				tryValues(offsetAlign*j, shift+((lengthAlign*(i+1))-1), shift+(lengthAlign*(i+1)))
				tryValues(offsetAlign*j, shift+(lengthAlign*(i+1)), shift+(lengthAlign*(i+1)))

				// Try some random values.
				for k := uint64(0); k < 25; k++ {
					rand := uint64(fastrand.Intn(int(lengthAlign)))
					rand++                            // move range from [0, lengthAlign) to [1, lengthAlign].
					rand += shift + (lengthAlign * i) // Move range into the range being tested.
					tryValues(offsetAlign*j, rand, shift+(lengthAlign*(i+1)))
				}
			}
		}
	}
}
