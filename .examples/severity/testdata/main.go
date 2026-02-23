package main

// TODO: refactor this function // want "TODO comment found"
func main() {
	if false {
		panic("unreachable") // want `call to panic\(\)`
	}
}
