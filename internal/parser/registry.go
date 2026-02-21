package parser

import "fmt"

// registry maps parser type names to factory functions
var registry = map[string]func() LogParser{}

// Register adds a parser factory to the registry.
// Called by each parser's init() function.
func Register(name string, factory func() LogParser) {
	registry[name] = factory
}

// Get returns a new instance of the parser for the given type name.
func Get(name string) (LogParser, error) {
	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown parser type: %q (available: %v)", name, AvailableParsers())
	}
	return factory(), nil
}

// AvailableParsers returns a list of registered parser names.
func AvailableParsers() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
