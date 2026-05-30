// Package codecs provides an example external custom-type codec for bencgen's
// Form B custom types. It exports Size<Name>/Marshal<Name>/Unmarshal<Name>
// functions matching benc's element-function signatures.
package codecs

import bstd "github.com/alvarolm/benc/std"

// Stamp is a stand-in for an external Go type (here, a wrapped millisecond
// timestamp) that bencgen cannot generate itself.
type Stamp struct {
	Millis int64
}

func SizeStamp(_ Stamp) int { return bstd.SizeInt64() }

func MarshalStamp(n int, b []byte, v Stamp) int {
	return bstd.MarshalInt64(n, b, v.Millis)
}

func UnmarshalStamp(n int, b []byte) (int, Stamp, error) {
	n, m, err := bstd.UnmarshalInt64(n, b)
	return n, Stamp{Millis: m}, err
}
