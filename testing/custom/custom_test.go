//go:generate bencgen --in ../schemas/custom.benc --out ./ --file ... --lang go

package custom

import (
	"reflect"
	"testing"

	"github.com/alvarolm/benc/testing/custom/codecs"
)

// Round-trips a Record exercising both custom-type forms:
//   - Form A alias scalars (Name, UserID), a []Name slice and a <Name, UserID> map
//   - Form B external-codec scalar (Stamp) and []Stamp slice
func TestCustomRoundTrip(t *testing.T) {
	original := Record{
		Name:    "alice",
		Id:      42,
		Created: codecs.Stamp{Millis: 1700000000000},
		Tags:    []Name{"admin", "ops"},
		Idx:     map[Name]UserID{"alice": 42, "bob": 7},
		Stamps:  []codecs.Stamp{{Millis: 1}, {Millis: 2}, {Millis: 3}},
	}

	buf := make([]byte, original.Size())
	original.Marshal(buf)

	var got Record
	if err := got.Unmarshal(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(original, got) {
		t.Fatalf("round-trip mismatch:\n original = %#v\n got      = %#v", original, got)
	}
}

// Empty slices/maps and zero custom values must also round-trip cleanly.
func TestCustomZeroValues(t *testing.T) {
	original := Record{}

	buf := make([]byte, original.Size())
	original.Marshal(buf)

	var got Record
	if err := got.Unmarshal(buf); err != nil {
		t.Fatal(err)
	}

	if got.Name != "" || got.Id != 0 || got.Created != (codecs.Stamp{}) {
		t.Fatalf("expected zero scalars, got %#v", got)
	}
	if len(got.Tags) != 0 || len(got.Idx) != 0 || len(got.Stamps) != 0 {
		t.Fatalf("expected empty collections, got %#v", got)
	}
}
