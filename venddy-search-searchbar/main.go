package main

import (
	loop "github.com/cameronvoglerolive/venddysearch/venddy-search-searchbar/loop"
)

func main() {
	if err := loop.Serve(); err != nil {
		panic(err)
	}
}
