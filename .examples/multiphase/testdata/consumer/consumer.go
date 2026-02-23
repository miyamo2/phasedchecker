package consumer

import "example.com/multiphase-testdata/api"

// Consume calls the deprecated OldGreet function.
func Consume() string {
	return api.OldGreet() // want "call to deprecated function OldGreet"
}
