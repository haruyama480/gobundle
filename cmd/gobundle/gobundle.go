package main

import (
	"flag"
	"fmt"

	"github.com/haruyama480/gobundler"
)

func init() {
	flag.Usage = func() {
		fmt.Println("Usage: gobundle <path>")
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		return
	}

	out, err := gobundler.Bundle(flag.Args()...)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(out)
}
