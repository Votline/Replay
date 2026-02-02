package main

import (
	"fmt"
	"os"

	"replay/internal/audio"
)

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

	f, err := os.OpenFile("file.bak", 0666, os.FileMode(os.O_RDWR))
	if err != nil {panic(err)}


	mode := os.Args[1]
	switch mode {
	case "record":
		acl.Record(f)
	case "replay":
		acl.Replay(f)
	default:
		fmt.Println("Usage: replay <mode>(record|replay")
	}
}
