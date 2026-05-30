//go:generate bencgen --in ../schemas/bench_bytes.benc --out ./ --file ... --lang go

package benchbytes

import "testing"

// 255 bytes of payload, shared by both the fixed-array and dynamic-bytes fields.
var payload = func() [255]byte {
	var a [255]byte
	for i := range a {
		a[i] = byte(i)
	}
	return a
}()

func newFixed() *FixedBlob { return &FixedBlob{Data: payload} }
func newDyn() *DynBlob     { return &DynBlob{Data: payload[:]} }

// 255 int32 elements, shared by the fixed-array and dynamic-slice fields.
var ints = func() [255]int32 {
	var a [255]int32
	for i := range a {
		a[i] = int32(i)
	}
	return a
}()

func newFixedInts() *FixedInts { return &FixedInts{Data: ints} }
func newDynInts() *DynInts     { return &DynInts{Data: ints[:]} }

// Encoded sizes — logged so the wire-size difference is visible.
// [255]byte: ArrayMap tag + len varint + 255 + 4-byte terminator.
// bytes:     Bytes tag    + len varint + 255 (no terminator).
func TestEncodedSizes(t *testing.T) {
	t.Logf("[255]byte  FixedBlob.Size() = %d bytes", newFixed().Size())
	t.Logf("bytes      DynBlob.Size()   = %d bytes", newDyn().Size())
	t.Logf("[255]int32 FixedInts.Size() = %d bytes", newFixedInts().Size())
	t.Logf("[]int32    DynInts.Size()   = %d bytes", newDynInts().Size())
}

func BenchmarkFixed255Marshal(b *testing.B) {
	v := newFixed()
	buf := make([]byte, v.Size())
	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Marshal(buf)
	}
}

func BenchmarkBytes255Marshal(b *testing.B) {
	v := newDyn()
	buf := make([]byte, v.Size())
	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Marshal(buf)
	}
}

func BenchmarkFixed255Unmarshal(b *testing.B) {
	v := newFixed()
	buf := make([]byte, v.Size())
	v.Marshal(buf)
	b.SetBytes(int64(len(buf)))
	var out FixedBlob // reused across iterations (realistic decode-into pattern)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := out.Unmarshal(buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBytes255Unmarshal(b *testing.B) {
	v := newDyn()
	buf := make([]byte, v.Size())
	v.Marshal(buf)
	b.SetBytes(int64(len(buf)))
	var out DynBlob // reused across iterations (realistic decode-into pattern)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := out.Unmarshal(buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFixedInts255Marshal(b *testing.B) {
	v := newFixedInts()
	buf := make([]byte, v.Size())
	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Marshal(buf)
	}
}

func BenchmarkDynInts255Marshal(b *testing.B) {
	v := newDynInts()
	buf := make([]byte, v.Size())
	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Marshal(buf)
	}
}

func BenchmarkFixedInts255Unmarshal(b *testing.B) {
	v := newFixedInts()
	buf := make([]byte, v.Size())
	v.Marshal(buf)
	b.SetBytes(int64(len(buf)))
	var out FixedInts // reused across iterations (realistic decode-into pattern)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := out.Unmarshal(buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDynInts255Unmarshal(b *testing.B) {
	v := newDynInts()
	buf := make([]byte, v.Size())
	v.Marshal(buf)
	b.SetBytes(int64(len(buf)))
	var out DynInts // reused across iterations (realistic decode-into pattern)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := out.Unmarshal(buf); err != nil {
			b.Fatal(err)
		}
	}
}
