package suid

import (
	"fmt"
	"testing"
)

func TestSuid(t *testing.T) {
	s := NewA()
	fmt.Println(s.Desc())
}
