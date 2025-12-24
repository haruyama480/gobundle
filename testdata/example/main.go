package main

import (
	"fmt"

	"example.com/mypackage/embedding"
	"example.com/mypackage/parent"
	"rsc.io/quote"
)

func main() {
	fmt.Println(parent.ParentFunc())
	fmt.Println(quote.Hello())
	embedding.Exec()
}
