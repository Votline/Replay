package main

import (
	"fmt"
	"os"

	"replay/internal/audio"
)

type debugWriter struct {}

func (d debugWriter) Write(p []byte) (int, error) {
	fmt.Printf("%s\n", string(p))
	return 0, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: replay <mode>(record|replay")
		return
	}

	acl, err := audio.Init()
	if err != nil {
		fmt.Printf("Init audio error: %s\n", err.Error())
		return
	}

	w := debugWriter{}

	mode := os.Args[1]
	switch mode {
	case "record":
		acl.Record(w)
	default:
		fmt.Println("Usage: replay <mode>(record|replay")
	}
}
