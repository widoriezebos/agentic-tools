package adapter

// Shared command-test helpers (placement audit, item 17): helpers used
// across runtimes live in a neutral test file, never a runtime's.

func fakeEnv(pairs map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := pairs[name]
		return value, ok
	}
}
