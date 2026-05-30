//go:generate bencgen --in ../schemas/fixed_arrays.benc --out ./ --file ... --lang go

package fixedarrays

import (
	"reflect"
	"testing"

	"github.com/alvarolm/benc"
	bstd "github.com/alvarolm/benc/std"
)

// Round-trips a Shapes exercising every fixed-array element kind:
// [16]byte (memcpy fast path), [4]int32 (fixed-width), [3]string (variable),
// [2]Point (container), [3]Name (custom), and [][3]int32 (slice of fixed arrays).
func TestFixedArrayRoundTrip(t *testing.T) {
	original := Shapes{
		Hash:    [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Quad:    [4]int32{-1, 2, -3, 2147483647},
		Labels:  [3]string{"alpha", "", "gamma"},
		Points:  [2]Point{{X: 1, Y: 2}, {X: -3, Y: 4}},
		Aliases: [3]Name{"root", "admin", ""},
		Matrix:  [][3]int32{{1, 2, 3}, {4, 5, 6}},
	}

	buf := make([]byte, original.Size())
	original.Marshal(buf)

	var got Shapes
	if err := got.Unmarshal(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(original, got) {
		t.Fatalf("round-trip mismatch:\n original = %#v\n got      = %#v", original, got)
	}
}

// Zero values must round-trip cleanly. (The dynamic Matrix slice decodes to a
// non-nil empty slice — benc's standard slice behavior — so it is checked by length
// rather than DeepEqual against a nil slice.)
func TestFixedArrayZeroValues(t *testing.T) {
	original := Shapes{}

	buf := make([]byte, original.Size())
	original.Marshal(buf)

	var got Shapes
	if err := got.Unmarshal(buf); err != nil {
		t.Fatal(err)
	}

	if got.Hash != ([16]byte{}) || got.Quad != ([4]int32{}) ||
		got.Labels != ([3]string{}) || got.Points != ([2]Point{}) ||
		got.Aliases != ([3]Name{}) {
		t.Fatalf("expected zero fixed arrays, got %#v", got)
	}
	if len(got.Matrix) != 0 {
		t.Fatalf("expected empty Matrix, got %#v", got.Matrix)
	}
}

// A payload whose encoded length differs from N must fail with ErrInvalidSize,
// not panic or corrupt — for both the generic and byte helpers.
func TestFixedArrayLengthMismatch(t *testing.T) {
	// Encode a 5-element int32 sequence, decode into a [3]int32.
	src := []int32{1, 2, 3, 4, 5}
	buf := make([]byte, bstd.SizeFixedSlice(src, bstd.SizeInt32()))
	bstd.MarshalSlice(0, buf, src, bstd.MarshalInt32)

	var arr [3]int32
	if _, err := bstd.UnmarshalFixedArray(0, buf, arr[:], bstd.UnmarshalInt32); err != benc.ErrInvalidSize {
		t.Fatalf("UnmarshalFixedArray: expected ErrInvalidSize, got %v", err)
	}

	// Same for the byte fast path: 5 bytes decoded into a [3]byte.
	five := []byte{1, 2, 3, 4, 5}
	bbuf := make([]byte, bstd.SizeFixedSlice(five, bstd.SizeByte()))
	bstd.MarshalFixedByteArray(0, bbuf, five)

	var barr [3]byte
	if _, err := bstd.UnmarshalFixedByteArray(0, bbuf, barr[:]); err != benc.ErrInvalidSize {
		t.Fatalf("UnmarshalFixedByteArray: expected ErrInvalidSize, got %v", err)
	}
}
