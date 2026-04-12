// Package assets embeds static files bundled into the conch binary.
package assets

import "embed"

//go:embed lsp.json
var KiroLSPConfig []byte

//go:embed agents/executor.json
var KiroExecutorAgent []byte

//go:embed agents/implementor.json
var KiroImplementorAgent []byte

//go:embed agents/planning.json
var KiroPlanningAgent []byte

//go:embed skills/CONCH_TASK/SKILL.md
var ConchTaskSkill []byte

//go:embed agents
var Agents embed.FS

//go:embed skills
var Skills embed.FS
