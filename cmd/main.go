package main

import (
	"fmt"

	"github.com/sutext/go/suid"
)

type User struct {
	ID   suid.SUID
	Name string
}

func main() {
	user := User{ID: suid.New(), Name: "John"}
	fmt.Printf("User ID: %d\n", user.ID.Value())
	// Code to execute the SUID binary
}
