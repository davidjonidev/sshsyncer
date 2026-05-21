package config

// RequireSetup loads the global config or returns the "run setup first" error.
func RequireSetup() (Global, error) {
	return LoadGlobal()
}
