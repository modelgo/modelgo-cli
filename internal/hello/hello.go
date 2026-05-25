// Package hello implements the demo `modelgo-cli hello` subcommand.
package hello

import "fmt"

// Greet returns "Hello, <name>!", defaulting to "world" when name is empty.
func Greet(name string) string {
	if name == "" {
		name = "world"
	}
	return fmt.Sprintf("Hello, %s!", name)
}
