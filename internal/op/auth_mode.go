package op

import "os"

// AuthMode describes how the `op` CLI authenticates.
type AuthMode int

const (
	AuthModeUnknown AuthMode = iota
	AuthModeDesktopApp
	AuthModeServiceAccount
)

func (m AuthMode) String() string {
	switch m {
	case AuthModeDesktopApp:
		return "desktop app"
	case AuthModeServiceAccount:
		return "service account"
	}
	return "unknown"
}

// DetectAuthMode infers which auth mode the op CLI will use.
// Service-account mode is selected when OP_SERVICE_ACCOUNT_TOKEN is set,
// regardless of whether the desktop app is also installed.
func DetectAuthMode() AuthMode {
	if os.Getenv("OP_SERVICE_ACCOUNT_TOKEN") != "" {
		return AuthModeServiceAccount
	}
	if home := os.Getenv("HOME"); home != "" {
		if _, err := os.Stat(home + "/Applications/1Password.app"); err == nil {
			return AuthModeDesktopApp
		}
	}
	if _, err := os.Stat("/Applications/1Password.app"); err == nil {
		return AuthModeDesktopApp
	}
	// Linux + manual signin: treat as desktop-equivalent for capability purposes.
	return AuthModeDesktopApp
}
