// Package kiro implements the harness.Harness interface for kiro-cli.
package kiro

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/FreezingSnail/conch/internal/assets"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/harness"
)

// compile-time interface check
var _ harness.Harness = Kiro{}

// Kiro implements harness.Harness for kiro-cli.
type Kiro struct{}

func (k Kiro) Name() string { return "kiro" }

func (k Kiro) Interactive() *exec.Cmd {
	return exec.Command("kiro-cli", "chat")
}

func (k Kiro) InteractiveWithAgent(agent, prompt, dir string) *exec.Cmd {
	cmd := exec.Command("kiro-cli", "chat", "--agent", agent, "--trust-all-tools", prompt)
	cmd.Dir = dir
	return cmd
}

func (k Kiro) Background(prompt string) *exec.Cmd {
	return exec.Command("kiro-cli", "chat", "--no-interactive", "--trust-all-tools", prompt)
}

func (k Kiro) BackgroundWithAgent(agent, prompt, dir string) *exec.Cmd {
	cmd := exec.Command("kiro-cli", "chat", "--no-interactive", "--trust-all-tools", "--agent", agent, prompt)
	cmd.Dir = dir
	return cmd
}

// CLICommand returns the shell command string for use in tmux invocations.
// If agent is empty, returns the bare interactive command.
func (k Kiro) CLICommand(agent, prompt string) string {
	if agent == "" {
		return "kiro-cli chat"
	}
	return fmt.Sprintf("kiro-cli chat --agent %s --trust-all-tools %q", agent, prompt)
}

// SeedWorktree writes embedded kiro config files into the worktree.
// Non-fatal: worktree is still usable without them.
func (k Kiro) SeedWorktree(worktreePath, slugMode string) {
	settingsDir := filepath.Join(worktreePath, ".kiro", "settings")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return
	}
	os.WriteFile(filepath.Join(settingsDir, "lsp.json"), assets.KiroLSPConfig, 0o644) //nolint:errcheck

	agentsDir := filepath.Join(worktreePath, ".kiro", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return
	}
	os.WriteFile(filepath.Join(agentsDir, "implementor.json"), injectSlugMode(assets.KiroImplementorAgent, slugMode), 0o644) //nolint:errcheck
	os.WriteFile(filepath.Join(agentsDir, "planning.json"), injectSlugMode(assets.KiroPlanningAgent, slugMode), 0o644)       //nolint:errcheck
	os.WriteFile(filepath.Join(agentsDir, "default.json"), injectSlugMode(assets.KiroDefaultAgent, slugMode), 0o644)         //nolint:errcheck
	os.WriteFile(filepath.Join(agentsDir, "pr-reviewer.json"), injectSlugMode(assets.KiroPRReviewerAgent, slugMode), 0o644)  //nolint:errcheck

	for name, data := range map[string][]byte{"slug": assets.SlugSkill, "slugdd": assets.SlugddSkill, "CONCH_TASK": assets.ConchTaskSkill} {
		dir := filepath.Join(worktreePath, ".kiro", "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		os.WriteFile(filepath.Join(dir, "SKILL.md"), data, 0o644) //nolint:errcheck
	}
}

// FindSession queries kiro's sqlite for the most recent session created in cwd
// after the given time. Returns the conversation UUID or empty string if not found.
func (k Kiro) FindSession(cwd string, after time.Time) (string, error) {
	return FindSessionByCwd(cwd, after)
}

// PlanningPrompt formats the planning context string passed to kiro-cli.
func (k Kiro) PlanningPrompt(ticketNum string, id int64, title, idea string) string {
	s := fmt.Sprintf("[CONCH PLANNING] ticket:%s id:%d", ticketNum, id)
	if title != "" {
		s += "\ntitle: " + title
	}
	if idea != "" {
		s += "\nidea: " + idea
	}
	return s
}

// ReplanPrompt builds the prompt for a replanning session.
func (k Kiro) ReplanPrompt(ticketNum string, id int64, title, desc string, notes []db.FeedbackNote) string {
	s := fmt.Sprintf("[CONCH PLANNING] ticket:%s id:%d", ticketNum, id)
	if title != "" {
		s += "\ntitle: " + title
	}
	if desc != "" {
		s += "\nidea: " + desc
	}
	if len(notes) == 0 {
		return s
	}
	lines := make([]string, len(notes))
	for i, n := range notes {
		lines[i] = fmt.Sprintf("- %s %s: %s", n.FilePath, n.HunkHeader, n.Body)
	}
	return s + "\n\nAdditional context (feedback notes from previous burrow session):\n" + strings.Join(lines, "\n")
}

// ImplementorPrompt builds the prompt passed to an implementor session.
func (k Kiro) ImplementorPrompt(task db.Task) string {
	return fmt.Sprintf("[CONCH TASK] task_id:%d\ntitle:%s\n\n%s", task.ID, task.Title, task.Body)
}

// PRReviewPrompt builds the prompt passed to the pr-reviewer agent.
func (k Kiro) PRReviewPrompt(prNum int, repo, diff string) string {
	return fmt.Sprintf("[CONCH PR REVIEW] pr:%d repo:%s\n\n%s", prNum, repo, diff)
}

// injectSlugMode prepends a slug activation preamble to the agent's prompt field.
func injectSlugMode(b []byte, mode string) []byte {
	if mode == "off" {
		return b
	}
	var agent map[string]interface{}
	if err := json.Unmarshal(b, &agent); err != nil {
		return b
	}
	prefix := slugPreamble(mode)
	if agent["name"] == "planning" {
		prefix += string(assets.SlugddSkill) + "\n\n"
	}
	if p, ok := agent["prompt"].(string); ok {
		agent["prompt"] = prefix + p
	}
	out, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		return b
	}
	return out
}

func slugPreamble(mode string) string {
	rules := "Respond terse like smart slug. All technical substance stay. Only fluff die. " +
		"Drop articles, filler (just/really/basically/actually/simply), pleasantries, hedging. " +
		"Fragments permitted. Short synonyms. Technical terms exact. Code blocks normal. " +
		"This mode MUST remain active every response."
	switch mode {
	case "lite":
		rules += " Keep articles and full sentences. Professional but tight."
	case "slugineer":
		rules += " Abbreviate (DB/auth/config/req/res/fn/impl). Strip conjunctions. Use arrows for causality (X → Y). One word when sufficient."
	}
	return "## Communication Style\n\nSlug mode: " + mode + ". " + rules + "\n\n"
}
