//go:generate protoc --go_out=. --go_opt=paths=source_relative -I . bench.proto

// Package protobench compares benc fixed-size arrays against the latest Go protobuf
// (google.golang.org/protobuf) for equivalent payloads:
//
//	benc [255]byte  (benchbytes.FixedBlob)  vs  protobuf bytes           (Blob)
//	benc [255]int32 (benchbytes.FixedInts)  vs  protobuf repeated sfixed32 (Ints)
//
// It lives in its own module so the protobuf dependency stays out of the core benc module.
package protobench

import (
	"testing"

	"github.com/alvarolm/benc/testing/benchbytes"
	"google.golang.org/protobuf/proto"
)

var payload = func() [255]byte {
	var a [255]byte
	for i := range a {
		a[i] = byte(i)
	}
	return a
}()

var ints = func() [255]int32 {
	var a [255]int32
	for i := range a {
		a[i] = int32(i)
	}
	return a
}()

// TestSizes logs the encoded size of each representation.
func TestSizes(t *testing.T) {
	bb := &benchbytes.FixedBlob{Data: payload}
	bi := &benchbytes.FixedInts{Data: ints}
	pb := &Blob{Data: payload[:]}
	pi := &Ints{Data: ints[:]}
	t.Logf("bytes:  benc [255]byte  = %4d B  |  protobuf bytes            = %4d B", bb.Size(), proto.Size(pb))
	t.Logf("int32:  benc [255]int32 = %4d B  |  protobuf repeated sfixed32 = %4d B", bi.Size(), proto.Size(pi))
}

// ---------------------------------------------------------------- bytes / [255]byte

func BenchmarkBencBytesMarshal(b *testing.B) {
	v := &benchbytes.FixedBlob{Data: payload}
	buf := make([]byte, v.Size())
	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Marshal(buf)
	}
}

func BenchmarkPbBytesMarshal(b *testing.B) {
	m := &Blob{Data: payload[:]}
	var opts proto.MarshalOptions
	buf, _ := opts.MarshalAppend(nil, m) // warm up capacity
	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		if buf, err = opts.MarshalAppend(buf[:0], m); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBencBytesUnmarshal(b *testing.B) {
	v := &benchbytes.FixedBlob{Data: payload}
	buf := make([]byte, v.Size())
	v.Marshal(buf)
	b.SetBytes(int64(len(buf)))
	var out benchbytes.FixedBlob
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := out.Unmarshal(buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPbBytesUnmarshal(b *testing.B) {
	buf, _ := proto.Marshal(&Blob{Data: payload[:]})
	b.SetBytes(int64(len(buf)))
	out := &Blob{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proto.Reset(out)
		if err := proto.Unmarshal(buf, out); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------- int32 / [255]int32

func BenchmarkBencIntsMarshal(b *testing.B) {
	v := &benchbytes.FixedInts{Data: ints}
	buf := make([]byte, v.Size())
	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Marshal(buf)
	}
}

func BenchmarkPbIntsMarshal(b *testing.B) {
	m := &Ints{Data: ints[:]}
	var opts proto.MarshalOptions
	buf, _ := opts.MarshalAppend(nil, m) // warm up capacity
	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		if buf, err = opts.MarshalAppend(buf[:0], m); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBencIntsUnmarshal(b *testing.B) {
	v := &benchbytes.FixedInts{Data: ints}
	buf := make([]byte, v.Size())
	v.Marshal(buf)
	b.SetBytes(int64(len(buf)))
	var out benchbytes.FixedInts
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := out.Unmarshal(buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPbIntsUnmarshal(b *testing.B) {
	buf, _ := proto.Marshal(&Ints{Data: ints[:]})
	b.SetBytes(int64(len(buf)))
	out := &Ints{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proto.Reset(out)
		if err := proto.Unmarshal(buf, out); err != nil {
			b.Fatal(err)
		}
	}
}
