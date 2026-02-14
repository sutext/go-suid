package suid

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGUIDEncode(t *testing.T) {
	_seq.Add(math.MaxUint64 - 20)
	for range 50 {
		fmt.Printf("seq: %d\n", _seq.Add(1)%MAX_SEQ)
	}

	var last time.Time
	for range 10 {
		now := time.Now()
		if !last.IsZero() {
			diff := now.Sub(last)
			fmt.Printf("Delta: %v (%d ns)\n", diff, diff.Nanoseconds())
		}
		last = now
	}
	tm := time.UnixMicro(0x3f_ffff_ffff_ffff)
	fmt.Printf("time: %v,now: %d\n", tm, time.Now().UnixMicro())
	id := New(1)
	t.Log(id.Description())
	id2, err := Parse(id.String())
	if err != nil {
		t.Error(err)
	}
	t.Log(id2.Description())
	t.Log(id2)
	if id != id2 {
		t.Error("not equal")
	}
}

type GUser struct {
	ID   GUID
	Name string
	Age  int
}

func TestGUIDJson(t *testing.T) {
	u := GUser{
		ID:   New(),
		Name: "Alice",
		Age:  25,
	}
	b, err := json.Marshal(u)
	if err != nil {
		t.Error(err)
	}
	t.Log(string(b))
	nu := GUser{}
	err = json.Unmarshal(b, &nu)
	if err != nil {
		t.Error(err)
	}
	t.Log(nu.ID.String())
	if !nu.ID.Verify() {
		t.Error("not verify")
	}
	if u != nu {
		t.Error("not equal")
	}
}
func TestGUIDConcurencey(t *testing.T) {
	var suids sync.Map
	t1 := time.Now()
	max := 0x7ffff
	var wg sync.WaitGroup
	wg.Go(func() {
		for range max {
			id := New()
			suids.Store(id.String(), id)
		}
	})
	wg.Go(func() {
		for range max {
			id := New()
			suids.Store(id.String(), id)
		}
	})
	wg.Go(func() {
		for range max {
			id := New()
			suids.Store(id.String(), id)
		}
	})
	wg.Wait()
	t2 := time.Now()
	fmt.Println("time used:", t2.Sub(t1))
	var len int
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
func BenchmarkGUIDEncode(b *testing.B) {
	b.Run("NewGUID", func(b *testing.B) {
		for b.Loop() {
			_ = New().String()
		}
	})
	b.Run("NewUUID", func(b *testing.B) {
		for b.Loop() {
			u, _ := uuid.NewV7()
			_ = u.String()
		}
	})
	b.Run("GUID", func(b *testing.B) {
		for b.Loop() {
			str := New().String()
			Parse(str)
		}
	})
	b.Run("UUID", func(b *testing.B) {
		for b.Loop() {
			u, _ := uuid.NewV7()
			uuid.Parse(u.String())
		}
	})
}
