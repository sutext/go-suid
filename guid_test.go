package suid

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUUIDEncode(t *testing.T) {
	var last time.Time
	for i := 0; i < 10; i++ {
		now := time.Now()
		if !last.IsZero() {
			diff := now.Sub(last)
			fmt.Printf("Delta: %v (%d ns)\n", diff, diff.Nanoseconds())
		}
		last = now
	}
	fmt.Printf("time: %v,timeWidth: %d\n", time.UnixMicro(0x7f_ffff_ffff_ffff), bitWidth(0x7f_ffff_ffff_ffff))
	id := NewGUID()
	t.Log(HostID())
	t.Log(id.Description())
	id2, err := ParseGUID(id.String())
	if err != nil {
		t.Error(err)
	}
	t.Log(id2.Description())
	t.Log(id2)
	if id != id2 {
		t.Error("not equal")
	}
}
func TestUUIDConcurencey(t *testing.T) {
	var suids sync.Map
	t1 := time.Now()
	max := MAX_SEQ
	var wg sync.WaitGroup
	wg.Go(func() {
		for range max {
			id := NewGUID()
			suids.Store(id.String(), id)
		}
	})
	wg.Go(func() {
		for range max {
			id := NewGUID()
			suids.Store(id.String(), id)
		}
	})
	wg.Go(func() {
		for range max {
			id := NewGUID()
			suids.Store(id.String(), id)
		}
	})
	wg.Wait()
	t2 := time.Now()
	fmt.Println("time used:", t2.Sub(t1))
	var len int64
	suids.Range(func(key, value any) bool {
		len++
		return true
	})
	t3 := time.Now()
	fmt.Println("time used:", t3.Sub(t2))
	if len != max*3 {
		t.Errorf("len of suids:%d is not equal to max:%d", len, max*3)
	}
}
func BenchmarkUUIDEncode(b *testing.B) {
	b.Run("NewGUID", func(b *testing.B) {
		for b.Loop() {
			_ = NewGUID().String()
		}
	})
	b.Run("NewUUID", func(b *testing.B) {
		for b.Loop() {
			u, _ := uuid.NewV7()
			_ = u.String()
		}
	})
}
