//go:build !windows

package config

// Non-Windows stub for credential store operations.
// On Linux/macOS the OS credential vault is not used; secrets come entirely
// from environment variables (or Docker secrets mounted as env vars).

// loadFromCredentialStore always returns ("", nil) on non-Windows platforms,
// causing Load() to fall through to the env-var path.
func loadFromCredentialStore(_ string) (string, error) {
	return "", nil
}

// StoreDesktopCredentials is a no-op on non-Windows platforms.
func StoreDesktopCredentials(_, _, _ string) error {
	return nil
}
