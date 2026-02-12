package suid

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

type User struct {
	ID   SUID
	Name string
	Age  int
}

func TestEncode(t *testing.T) {

	id := New()
	fmt.Println(id.Host())
	t.Log(id)
	t.Log(id.Integer())
	str := id.String()
	id2, err := FromString(str)
	t.Log(id2)
	if err != nil {
		t.Error(err)
	}
	if id != id2 {
		t.Error("not equal")
	}
}
func TestJson(t *testing.T) {
	u := User{
		ID:   New(),
		Name: "Alice",
		Age:  25,
	}
	b, err := json.Marshal(u)
	if err != nil {
		t.Error(err)
	}
	t.Log(string(b))
	nu := User{}
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
func TestConcurencey(t *testing.T) {
	var suids sync.Map
	t1 := time.Now()
	max := MAX_SEQ / 3
	var wg sync.WaitGroup
	wg.Go(func() {
		for range max {
			id := New()
			suids.Store(id.Integer(), id)
		}
	})
	wg.Go(func() {
		for range max {
			id := New()
			suids.Store(id.Integer(), id)
		}
	})
	wg.Go(func() {
		for range max {
			id := New()
			suids.Store(id.Integer(), id)
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

func BenchmarkGenerate(b *testing.B) {
	id := New()
	str := id.String()
	i := id.Integer()
	b.ReportAllocs()
	b.Run("New", func(b *testing.B) {
		for b.Loop() {
			New()
		}
	})
	b.Run("NewFromInteger", func(b *testing.B) {
		for b.Loop() {
			FromInteger(i)
		}
	})
	b.Run("NewFromString", func(b *testing.B) {
		for b.Loop() {
			FromString(str)
		}
	})
}

func BenchmarkJSON(b *testing.B) {
	u := User{
		ID:   New(),
		Name: "Alice",
		Age:  25,
	}
	b.ReportAllocs()
	b.Run("Marshal", func(b *testing.B) {
		for b.Loop() {
			_, _ = json.Marshal(u)
		}
	})
	data, _ := json.Marshal(u)
	b.Log(string(data))
	b.Run("Unmarshal", func(b *testing.B) {
		for b.Loop() {
			var nu User
			_ = json.Unmarshal(data, &nu)
		}
	})
}
