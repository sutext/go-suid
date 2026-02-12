package suid

import (
	"testing"
)

func TestUUIDEncode(t *testing.T) {
	id := NewUUID()
	t.Log(HostID())
	t.Log(id.Description())
	id2, err := ParseUUID(id.String())
	if err != nil {
		t.Error(err)
	}
	t.Log(id2.Description())
	if id != id2 {
		t.Error("not equal")
	}
}
