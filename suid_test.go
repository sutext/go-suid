package suid

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

type User struct {
	ID   SUID
	Name string
	Age  int
}

var suids = make(map[int64]SUID)

func TestJson(t *testing.T) {
	fmt.Println(time.Unix(1745400000, 0).Format(time.DateTime))
	fmt.Println("TestJson")
	for i := 0; i < 8; i++ {
		u := User{
			ID:   New(int64(i)),
			Name: "Alice",
			Age:  25,
		}
		b, _ := json.Marshal(u)
		t.Log(string(b))
		nu := User{}
		json.Unmarshal(b, &nu)
		t.Log(nu.ID.Description())
		if !nu.ID.Verify() {
			t.Error("not verify")
		}
		if u != nu {
			t.Error("not equal")
		}
	}
}
func TestConcurenceyChain(t *testing.T) {
	fmt.Println("TestConcurenceyChain")
	t1 := time.Now()
	ch := make(chan SUID)
	max := MAX_SEQ
	go func() {
		for range max {
			ch <- New(2)
		}
		close(ch)
	}()
	for id := range ch {
		suids[id.Int()] = id
	}
	t2 := time.Now()
	t.Log(t2.Sub(t1))
	if len(suids) != int(max) {
		t.Errorf("len of suids:%d is not equal to max:%d", len(suids), max)
	}
}
