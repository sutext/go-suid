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
	fmt.Println(SUID{})
	fmt.Println(time.Unix(0x1ffffffff, 0).UTC())
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
	for range 1 {
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
}
func TestConcurencey(t *testing.T) {
	fmt.Println(New())
	var suids sync.Map
	t1 := time.Now()
	max := MAX_SEQ
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
	len := 0
	suids.Range(func(key, value any) bool {
		// t.Log(key, value)
		len++
		return true
	})
	t3 := time.Now()
	fmt.Println("time used:", t3.Sub(t2))
	if len != int(max*3) {
		t.Errorf("len of suids:%d is not equal to max:%d", len, max)
	}
}
