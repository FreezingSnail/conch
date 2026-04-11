// Package assets embeds static files bundled into the conch binary.
package assets

import _ "embed"

//go:embed lsp.json
var KiroLSPConfig []byte
