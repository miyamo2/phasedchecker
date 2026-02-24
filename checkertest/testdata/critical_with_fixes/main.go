package main

var bar = 1 // want `renaming "bar" to "baz"` "test diagnostic"

func main() {
	_ = bar // want `renaming "bar" to "baz"`
}
