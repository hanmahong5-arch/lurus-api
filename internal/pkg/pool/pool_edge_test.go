package pool

import (
	"strings"
	"sync"
	"testing"
)

// --- Buffer: nil-guard, reuse, oversize-cap skip ---

func TestPutBuffer_Nil(t *testing.T) {
	// Must not panic on nil.
	PutBuffer(nil)
}

func TestPutBuffer_ResetOnReuse(t *testing.T) {
	buf := GetBuffer()
	buf.WriteString("dirty")
	PutBuffer(buf)
	// A subsequent GetBuffer must always hand back a reset buffer,
	// regardless of whether the pool returns the same object.
	buf2 := GetBuffer()
	if buf2.Len() != 0 {
		t.Fatalf("expected empty buffer after Get, got len=%d content=%q", buf2.Len(), buf2.String())
	}
	PutBuffer(buf2)
}

func TestPutBuffer_OversizeSkipped(t *testing.T) {
	buf := GetBuffer()
	// Grow the buffer beyond the 64KiB cap so PutBuffer takes the skip branch.
	buf.Write(make([]byte, 128*1024))
	if buf.Cap() <= 64*1024 {
		t.Fatalf("precondition: buffer cap must exceed 64KiB, got %d", buf.Cap())
	}
	PutBuffer(buf) // hits the oversize early-return; buffer is dropped, not pooled
	// The dropped buffer must not be returned; can't observe pool internals,
	// but we assert the next Get still yields a usable, empty buffer.
	next := GetBuffer()
	if next.Len() != 0 {
		t.Fatalf("expected empty buffer, got len=%d", next.Len())
	}
	PutBuffer(next)
}

// --- IntSlice: nil-guard, reset, oversize-cap skip ---

func TestPutIntSlice_Nil(t *testing.T) {
	PutIntSlice(nil)
}

func TestGetIntSlice_ResetLen(t *testing.T) {
	s := GetIntSlice()
	*s = append(*s, 7, 8, 9)
	PutIntSlice(s)
	s2 := GetIntSlice()
	if len(*s2) != 0 {
		t.Fatalf("expected len 0 after Get, got %d", len(*s2))
	}
	PutIntSlice(s2)
}

func TestPutIntSlice_OversizeSkipped(t *testing.T) {
	s := GetIntSlice()
	big := make([]int, 0, 2048) // cap > 1024
	*s = big
	if cap(*s) <= 1024 {
		t.Fatalf("precondition: cap must exceed 1024, got %d", cap(*s))
	}
	PutIntSlice(s) // oversize skip branch
	next := GetIntSlice()
	if len(*next) != 0 {
		t.Fatalf("expected len 0, got %d", len(*next))
	}
	PutIntSlice(next)
}

// --- StringBuilder: nil-guard, reset, oversize-cap skip ---

func TestPutStringBuilder_Nil(t *testing.T) {
	PutStringBuilder(nil)
}

func TestGetStringBuilder_ResetOnReuse(t *testing.T) {
	sb := GetStringBuilder()
	sb.WriteString("stale")
	PutStringBuilder(sb)
	sb2 := GetStringBuilder()
	if sb2.Len() != 0 {
		t.Fatalf("expected empty builder, got len=%d content=%q", sb2.Len(), sb2.String())
	}
	PutStringBuilder(sb2)
}

func TestPutStringBuilder_OversizeSkipped(t *testing.T) {
	sb := GetStringBuilder()
	// Grow past 64KiB so PutStringBuilder hits the skip branch.
	sb.WriteString(strings.Repeat("x", 128*1024))
	if sb.Cap() <= 64*1024 {
		t.Fatalf("precondition: builder cap must exceed 64KiB, got %d", sb.Cap())
	}
	PutStringBuilder(sb) // oversize early-return
	next := GetStringBuilder()
	if next.Len() != 0 {
		t.Fatalf("expected empty builder, got len=%d", next.Len())
	}
	PutStringBuilder(next)
}

// --- IntBoolMap: nil-guard, oversize (len) skip ---

func TestPutIntBoolMap_Nil(t *testing.T) {
	PutIntBoolMap(nil)
}

func TestPutIntBoolMap_OversizeSkipped(t *testing.T) {
	m := GetIntBoolMap()
	for i := 0; i < 1025; i++ { // len > 1024
		m[i] = true
	}
	if len(m) <= 1024 {
		t.Fatalf("precondition: map len must exceed 1024, got %d", len(m))
	}
	PutIntBoolMap(m) // oversize skip branch — huge map is dropped, not pooled
	// Next map from pool must be a cleared map.
	next := GetIntBoolMap()
	if len(next) != 0 {
		t.Fatalf("expected cleared map, got len=%d", len(next))
	}
	PutIntBoolMap(next)
}

// --- GenericSlice: full lifecycle (was 0% covered) ---

func TestGetGenericSlice(t *testing.T) {
	s := GetGenericSlice()
	if s == nil {
		t.Fatal("GetGenericSlice returned nil")
	}
	if len(*s) != 0 {
		t.Fatalf("expected fresh slice len 0, got %d", len(*s))
	}
	*s = append(*s, "a", 1, struct{}{})
	if len(*s) != 3 {
		t.Fatalf("expected len 3, got %d", len(*s))
	}
	PutGenericSlice(s)
	// Reuse must be reset to len 0.
	s2 := GetGenericSlice()
	if len(*s2) != 0 {
		t.Fatalf("expected len 0 after Get, got %d", len(*s2))
	}
	PutGenericSlice(s2)
}

func TestPutGenericSlice_Nil(t *testing.T) {
	PutGenericSlice(nil)
}

func TestPutGenericSlice_OversizeSkipped(t *testing.T) {
	s := GetGenericSlice()
	big := make([]interface{}, 0, 2048) // cap > 1024
	*s = big
	if cap(*s) <= 1024 {
		t.Fatalf("precondition: cap must exceed 1024, got %d", cap(*s))
	}
	PutGenericSlice(s) // oversize skip branch
	next := GetGenericSlice()
	if len(*next) != 0 {
		t.Fatalf("expected len 0, got %d", len(*next))
	}
	PutGenericSlice(next)
}

// --- Concurrency: exercise all pools under -race, assert no corruption/leak ---

func TestPools_ConcurrentGetPut(t *testing.T) {
	const goroutines = 32
	const iters = 500
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				buf := GetBuffer()
				if buf.Len() != 0 {
					t.Errorf("buffer not reset: len=%d", buf.Len())
					return
				}
				buf.WriteString("payload")
				PutBuffer(buf)

				is := GetIntSlice()
				if len(*is) != 0 {
					t.Errorf("int slice not reset: len=%d", len(*is))
					return
				}
				*is = append(*is, 1, 2, 3)
				PutIntSlice(is)

				sb := GetStringBuilder()
				if sb.Len() != 0 {
					t.Errorf("builder not reset: len=%d", sb.Len())
					return
				}
				sb.WriteString("x")
				PutStringBuilder(sb)

				m := GetIntBoolMap()
				if len(m) != 0 {
					t.Errorf("map not cleared: len=%d", len(m))
					return
				}
				m[i] = true
				PutIntBoolMap(m)

				gs := GetGenericSlice()
				if len(*gs) != 0 {
					t.Errorf("generic slice not reset: len=%d", len(*gs))
					return
				}
				*gs = append(*gs, i)
				PutGenericSlice(gs)
			}
		}()
	}
	wg.Wait()
}
