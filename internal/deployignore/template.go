package deployignore

import _ "embed"

// Default is the embedded default .deployignore content written by `sshsyncer init`.
//
//go:embed default.ignore
var Default string
