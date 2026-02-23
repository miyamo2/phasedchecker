package consumer

import "github.com/miyamo2/phasedchecker/examples/multiphase/target/api"

// Consume calls the deprecated OldGreet function.
func Consume() string {
	return api.OldGreet()
}
