package main

import (
	"log"
	"os"

	"github.com/seamanticus/amitty/cmd"
)

func main() {
	if err := cmd.Run(); err != nil {
		log.SetOutput(os.Stderr)
		log.Fatal(err)
	}
}
