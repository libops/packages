// Package config captures process configuration at the CLI boundary.
package config

import (
	"os"
	"strings"
)

type Environment map[string]string

func Snapshot() Environment {
	values := Environment{}
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	return values
}
func (e Environment) Get(key string) string { return e[key] }
func (e Environment) Copy() map[string]string {
	values := map[string]string{}
	for key, value := range e {
		values[key] = value
	}
	return values
}
