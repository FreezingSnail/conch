package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	glamour "charm.land/glamour/v2"
	"github.com/FreezingSnail/conch/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

type mantleSection int

const (
	mantleAgents   mantleSection = iota
	mantleDocs                   // embedded README
	mantleSettings               // conch config
	mantleSkills                 // skill viewer
)

var mantleSectionNames = []string{"Agents", "Docs", "Settings", "Skills"}

type mantleView struct {
	section  mantleSection
	agents   []string
	skills   []string
	cursor   int
	content  string // raw content (for scroll line count)
	rendered string // glamour-rendered content for display
	title    string // reader title
	scroll   int
	height   int
	w        int
	// docs / settings loaded lazily
	readme   string
	settings string
}

func (v mantleView) width() int {
	if v.w > 0 {
		return v.w
	}
	return 80
}

type mantleLoadedMsg struct {
	agents   []string
	skills   []string
	readme   string
	settings string
}

func newMantleView() mantleView { return mantleView{} }

func (v mantleView) Init() tea.Cmd {
	return func() tea.Msg {
		// agents
		base := filepath.Join(os.Getenv("HOME"), ".conch", "agents")
		entries, err := os.ReadDir(base)
		if err != nil {
			entries, _ = os.ReadDir("tooling/agents")
			base = "tooling/agents"
		}
		var agents []string
		for _, e := range entries {
			if e.IsDir() {
				if _, err := os.Stat(filepath.Join(base, e.Name(), "SKILL.md")); err == nil {
					agents = append(agents, e.Name())
				}
			}
		}

		// skills
		skillsBase := filepath.Join(os.Getenv("HOME"), ".conch", "skills")
		skillEntries, err := os.ReadDir(skillsBase)
		if err != nil {
			skillEntries, _ = os.ReadDir("tooling/skills")
			skillsBase = "tooling/skills"
		}
		var skills []string
		for _, e := range skillEntries {
			if e.IsDir() {
				if _, err := os.Stat(filepath.Join(skillsBase, e.Name(), "SKILL.md")); err == nil {
					skills = append(skills, e.Name())
				}
			}
		}

		// readme — search upward from cwd for README.md
		readme := readFileUpward("README.md")

		// settings
		cfg, err := config.Load()
		var settingsStr string
		if err != nil {
			settingsStr = "error loading config: " + err.Error()
		} else {
			b, _ := json.MarshalIndent(cfg, "", "  ")
			settingsStr = fmt.Sprintf("config path: %s\n\n%s",
				filepath.Join(os.Getenv("HOME"), ".conch", "config.json"), string(b))
		}

		return mantleLoadedMsg{agents: agents, skills: skills, readme: readme, settings: settingsStr}
	}
}

func readFileUpward(name string) string {
	dir, _ := os.Getwd()
	for {
		p := filepath.Join(dir, name)
		if b, err := os.ReadFile(p); err == nil {
			return string(b)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "(not found)"
}

func (v mantleView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.height = msg.Height - 6
		v.w = msg.Width
	case mantleLoadedMsg:
		v.agents = msg.agents
		v.skills = msg.skills
		v.readme = msg.readme
		v.settings = msg.settings
	case mantleOpenMsg:
		v.content = msg.content
		v.title = msg.title
		v.scroll = 0
		v.rendered = renderMarkdown(msg.content, v.width())
	case tea.KeyMsg:
		if v.content != "" {
			return v.handleReaderKey(msg)
		}
		return v.handleNavKey(msg)
	}
	return v, nil
}

func (v mantleView) handleNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return v, pop()
	case "tab":
		v.section = (v.section + 1) % mantleSection(len(mantleSectionNames))
		v.cursor = 0
	case "shift+tab":
		v.section = (v.section + mantleSection(len(mantleSectionNames)) - 1) % mantleSection(len(mantleSectionNames))
		v.cursor = 0
	case "up", "k":
		if v.cursor > 0 {
			v.cursor--
		}
	case "down", "j":
		if v.section == mantleAgents && v.cursor < len(v.agents)-1 {
			v.cursor++
		} else if v.section == mantleSkills && v.cursor < len(v.skills)-1 {
			v.cursor++
		}
	case "enter":
		switch v.section {
		case mantleAgents:
			if len(v.agents) == 0 {
				return v, nil
			}
			name := v.agents[v.cursor]
			return v, readSkillCmd(filepath.Join(os.Getenv("HOME"), ".conch", "agents"), "tooling/agents", name)
		case mantleSkills:
			if len(v.skills) == 0 {
				return v, nil
			}
			name := v.skills[v.cursor]
			return v, readSkillCmd(filepath.Join(os.Getenv("HOME"), ".conch", "skills"), "tooling/skills", name)
		case mantleDocs:
			v.content = v.readme
			v.title = "README"
			v.scroll = 0
			v.rendered = renderMarkdown(v.readme, v.width())
		case mantleSettings:
			v.content = v.settings
			v.title = "Settings"
			v.scroll = 0
			v.rendered = v.settings // plain text, no markdown rendering
		}
	}
	return v, nil
}

type mantleOpenMsg struct{ title, content string }

func readSkillCmd(primaryBase, fallbackBase, name string) tea.Cmd {
	return func() tea.Msg {
		p := filepath.Join(primaryBase, name, "SKILL.md")
		b, err := os.ReadFile(p)
		if err != nil {
			p = filepath.Join(fallbackBase, name, "SKILL.md")
			b, err = os.ReadFile(p)
		}
		if err != nil {
			return mantleOpenMsg{title: name, content: "error: " + err.Error()}
		}
		return mantleOpenMsg{title: name, content: string(b)}
	}
}

func (v mantleView) handleReaderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := strings.Split(v.rendered, "\n")
	switch msg.String() {
	case "esc", "q":
		v.content = ""
		v.rendered = ""
		v.title = ""
	case "up", "k":
		if v.scroll > 0 {
			v.scroll--
		}
	case "down", "j":
		if v.scroll < len(lines)-1 {
			v.scroll++
		}
	}
	return v, nil
}

func (v mantleView) View() string {
	if v.content != "" {
		return v.viewReader()
	}

	// tab bar
	s := "  Mantle\n\n"
	for i, name := range mantleSectionNames {
		if mantleSection(i) == v.section {
			s += fmt.Sprintf("  [%s]", name)
		} else {
			s += fmt.Sprintf("   %s ", name)
		}
		if i < len(mantleSectionNames)-1 {
			s += "  "
		}
	}
	s += "\n\n"

	switch v.section {
	case mantleAgents:
		if len(v.agents) == 0 {
			s += "  no agents found\n"
		} else {
			for i, name := range v.agents {
				cur := "  "
				if i == v.cursor {
					cur = "> "
				}
				s += fmt.Sprintf("%s%s\n", cur, name)
			}
		}
		s += "\n  ↑/↓ navigate  enter read  tab switch  esc back\n"
	case mantleDocs:
		s += "  README.md\n\n  enter to read\n\n  tab switch  esc back\n"
	case mantleSettings:
		s += "  conch config\n\n  enter to view\n\n  tab switch  esc back\n"
	case mantleSkills:
		if len(v.skills) == 0 {
			s += "  no skills found\n"
		} else {
			for i, name := range v.skills {
				cur := "  "
				if i == v.cursor {
					cur = "> "
				}
				s += fmt.Sprintf("%s%s\n", cur, name)
			}
		}
		s += "\n  ↑/↓ navigate  enter read  tab switch  esc back\n"
	}
	return s
}

func (v mantleView) viewReader() string {
	lines := strings.Split(v.rendered, "\n")
	pageH := v.height
	if pageH < 5 {
		pageH = 20
	}
	end := v.scroll + pageH
	if end > len(lines) {
		end = len(lines)
	}
	s := fmt.Sprintf("  Mantle — %s\n\n", v.title)
	s += strings.Join(lines[v.scroll:end], "\n")
	s += fmt.Sprintf("\n\n  line %d/%d   ↑/↓ scroll  esc back\n", v.scroll+1, len(lines))
	return s
}

func renderMarkdown(md string, width int) string {
	if width <= 0 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(glamour.WithEnvironmentConfig(), glamour.WithWordWrap(width-4))
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return out
}
