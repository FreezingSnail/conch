package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	glamour "charm.land/glamour/v2"
	"github.com/FreezingSnail/conch/internal/assets"
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

type agentDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Prompt      string          `json:"prompt"`
	Tools       []string        `json:"tools"`
	MCPServers  json.RawMessage `json:"mcpServers"`
	Resources   []string        `json:"resources"`
}

type mantleView struct {
	section  mantleSection
	agents   []agentDef
	skills   []string
	cursor   int
	content  string // raw content (for scroll line count)
	rendered string // glamour-rendered content for display
	title    string // reader title
	scroll   int
	w, h     int
	// docs / settings loaded lazily
	readme   string
	settings string
	// settings editor
	cfg     config.Config
	editing bool   // adding a new work_dir
	input   string // text being typed
	// search
	searchActive  bool
	searchQuery   string
	searchMatches []int // line indices with matches
	searchIdx     int   // current match index
}

func (v mantleView) width() int {
	if v.w > 0 {
		return v.w
	}
	return 80
}

type mantleLoadedMsg struct {
	agents   []agentDef
	skills   []string
	readme   string
	settings string
	cfg      config.Config
}

// mantleOpenDocsMsg instructs mantle to open the docs reader at a named section heading.
type mantleOpenDocsMsg struct{ section string }

func newMantleView() mantleView { return mantleView{} }

// Title implements Titler; used by the tab bar chrome.
func (v mantleView) Title() string { return "Mantle" }

// HelpLine implements Helper; returns context-sensitive keybinding hints.
func (v mantleView) HelpLine() string {
	if v.content != "" {
		if v.searchActive {
			return "type to search  enter/esc close search  n/N cycle matches"
		}
		return "↑/↓ scroll  / search  n/N cycle matches  esc back"
	}
	switch v.section {
	case mantleDocs:
		return "enter read  tab switch  esc back"
	case mantleSettings:
		if v.editing {
			return "enter confirm  esc cancel"
		}
		return "↑/↓ navigate  a add  d delete  s cycle-slug-mode  w toggle-wenyan  tab switch  esc back"
	default:
		return "↑/↓ navigate  enter read  tab switch  esc back"
	}
}

func (v mantleView) Init() tea.Cmd {
	return func() tea.Msg {
		// agents — load from embedded FS
		agentEntries, _ := assets.Agents.ReadDir("agents")
		var agents []agentDef
		for _, e := range agentEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			b, err := assets.Agents.ReadFile("agents/" + e.Name())
			if err != nil {
				continue
			}
			var a agentDef
			if json.Unmarshal(b, &a) == nil {
				agents = append(agents, a)
			}
		}

		// skills — load from embedded FS
		skillEntries, _ := assets.Skills.ReadDir("skills")
		var skills []string
		for _, e := range skillEntries {
			if e.IsDir() {
				if _, err := assets.Skills.Open("skills/" + e.Name() + "/SKILL.md"); err == nil {
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

		return mantleLoadedMsg{agents: agents, skills: skills, readme: readme, settings: settingsStr, cfg: cfg}
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
		v.h = msg.Height - 4 // match chrome body height
		v.w = msg.Width
	case mantleLoadedMsg:
		v.agents = msg.agents
		v.skills = msg.skills
		v.readme = msg.readme
		v.settings = msg.settings
		v.cfg = msg.cfg
	case mantleSavedMsg:
		v.cfg = msg.cfg
	case mantleOpenMsg:
		v.content = msg.content
		v.title = msg.title
		v.scroll = 0
		v.rendered = renderMarkdown(msg.content, v.width())
		v.clearSearch()
	case mantleOpenDocsMsg:
		// Ensure readme is loaded; if not yet, we'll handle after load.
		// If already loaded, open docs reader and scroll to section.
		if v.readme != "" && v.readme != "(not found)" {
			v.openDocsAtSection(msg.section)
		} else {
			// Store pending section in title temporarily; resolved after load.
			v.title = "##pending:" + msg.section
		}
	case tea.KeyMsg:
		if v.content != "" {
			return v.handleReaderKey(msg)
		}
		if v.section == mantleSettings && v.editing {
			return v.handleSettingsInput(msg)
		}
		return v.handleNavKey(msg)
	}
	// Handle pending deep-link after readme loads.
	if strings.HasPrefix(v.title, "##pending:") && v.readme != "" && v.readme != "(not found)" {
		section := strings.TrimPrefix(v.title, "##pending:")
		v.title = ""
		v.openDocsAtSection(section)
	}
	return v, nil
}

// openDocsAtSection opens the docs reader and scrolls to the first line matching "## <section>".
func (v *mantleView) openDocsAtSection(section string) {
	v.content = v.readme
	v.title = "README"
	v.rendered = renderMarkdown(v.readme, v.width())
	v.clearSearch()

	// Scroll to the heading line.
	heading := "## " + section
	lines := strings.Split(v.rendered, "\n")
	for i, line := range lines {
		if strings.Contains(line, section) && strings.Contains(strings.ToLower(line), strings.ToLower(heading[3:])) {
			v.scroll = i
			break
		}
	}
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
		} else if v.section == mantleSettings && v.cursor < len(v.cfg.WorkDirs)-1 {
			v.cursor++
		}
	case "enter":
		switch v.section {
		case mantleAgents:
			if len(v.agents) == 0 {
				return v, nil
			}
			a := v.agents[v.cursor]
			content := agentDetailContent(a)
			v.content = content
			v.title = a.Name
			v.scroll = 0
			v.rendered = renderMarkdown(content, v.width())
			v.clearSearch()
			return v, nil
		case mantleSkills:
			if len(v.skills) == 0 {
				return v, nil
			}
			name := v.skills[v.cursor]
			return v, readSkillCmd(name)
		case mantleDocs:
			v.content = v.readme
			v.title = "README"
			v.scroll = 0
			v.rendered = renderMarkdown(v.readme, v.width())
			v.clearSearch()
		case mantleSettings:
			v.content = v.settings
			v.title = "Settings"
			v.scroll = 0
			v.rendered = v.settings // plain text, no markdown rendering
			v.clearSearch()
		}
	case "a":
		if v.section == mantleSettings {
			v.editing = true
			v.input = ""
		}
	case "d":
		if v.section == mantleSettings && len(v.cfg.WorkDirs) > 0 {
			v.cfg.WorkDirs = append(v.cfg.WorkDirs[:v.cursor], v.cfg.WorkDirs[v.cursor+1:]...)
			if v.cursor >= len(v.cfg.WorkDirs) && v.cursor > 0 {
				v.cursor--
			}
			return v, saveConfigCmd(v.cfg)
		}
	case "s":
		if v.section == mantleSettings {
			modes := []string{"slugineer", "lite", "off"}
			cur := v.cfg.EffectiveSlugMode()
			next := modes[0]
			for i, m := range modes {
				if m == cur {
					next = modes[(i+1)%len(modes)]
					break
				}
			}
			v.cfg.SlugMode = next
			return v, saveConfigCmd(v.cfg)
		}
	case "w":
		if v.section == mantleSettings {
			v.cfg.WenyanMode = !v.cfg.WenyanMode
			return v, saveConfigCmd(v.cfg)
		}
	}
	return v, nil
}

type mantleOpenMsg struct{ title, content string }

// agentDetailContent builds a markdown document for an agent.
func agentDetailContent(a agentDef) string {
	var sb strings.Builder
	sb.WriteString("# " + a.Name + "\n\n")
	sb.WriteString("| Field | Value |\n|---|---|\n")
	sb.WriteString("| description | " + strings.ReplaceAll(a.Description, "\n", " ") + " |\n")
	if len(a.Tools) > 0 {
		sb.WriteString("| tools | " + strings.Join(a.Tools, ", ") + " |\n")
	}
	if len(a.Resources) > 0 {
		sb.WriteString("| resources | " + strings.Join(a.Resources, ", ") + " |\n")
	}
	if len(a.MCPServers) > 0 && string(a.MCPServers) != "null" {
		sb.WriteString("| mcpServers | " + strings.ReplaceAll(string(a.MCPServers), "\n", " ") + " |\n")
	}
	sb.WriteString("\n---\n\n")
	sb.WriteString(a.Prompt)
	return sb.String()
}

func readSkillCmd(name string) tea.Cmd {
	return func() tea.Msg {
		b, err := assets.Skills.ReadFile("skills/" + name + "/SKILL.md")
		if err != nil {
			return mantleOpenMsg{title: name, content: "error: " + err.Error()}
		}
		return mantleOpenMsg{title: name, content: string(b)}
	}
}

type mantleSavedMsg struct{ cfg config.Config }

func saveConfigCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		config.Save(cfg)
		return mantleSavedMsg{cfg: cfg}
	}
}

func (v mantleView) handleSettingsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.editing = false
		v.input = ""
	case "enter":
		if v.input != "" {
			v.cfg.WorkDirs = append(v.cfg.WorkDirs, v.input)
			v.cursor = len(v.cfg.WorkDirs) - 1
			v.editing = false
			v.input = ""
			return v, saveConfigCmd(v.cfg)
		}
	case "backspace":
		if len(v.input) > 0 {
			v.input = v.input[:len(v.input)-1]
		}
	default:
		if len(msg.String()) == 1 {
			v.input += msg.String()
		}
	}
	return v, nil
}

func (v mantleView) handleReaderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Search bar active: capture typing.
	if v.searchActive {
		switch msg.String() {
		case "esc", "enter":
			v.searchActive = false
		case "backspace":
			if len(v.searchQuery) > 0 {
				v.searchQuery = v.searchQuery[:len(v.searchQuery)-1]
				v.rebuildMatches()
			}
		default:
			if len(msg.String()) == 1 {
				v.searchQuery += msg.String()
				v.rebuildMatches()
			}
		}
		return v, nil
	}

	lines := strings.Split(v.rendered, "\n")
	switch msg.String() {
	case "esc", "q":
		v.content = ""
		v.rendered = ""
		v.title = ""
		v.clearSearch()
	case "up", "k":
		if v.scroll > 0 {
			v.scroll--
		}
	case "down", "j":
		if v.scroll < len(lines)-1 {
			v.scroll++
		}
	case "/":
		v.searchActive = true
		v.searchQuery = ""
		v.rebuildMatches()
	case "n":
		if len(v.searchMatches) > 0 {
			v.searchIdx = (v.searchIdx + 1) % len(v.searchMatches)
			v.scroll = v.searchMatches[v.searchIdx]
		}
	case "N":
		if len(v.searchMatches) > 0 {
			v.searchIdx = (v.searchIdx - 1 + len(v.searchMatches)) % len(v.searchMatches)
			v.scroll = v.searchMatches[v.searchIdx]
		}
	}
	return v, nil
}

// rebuildMatches scans rendered lines for searchQuery and updates searchMatches.
func (v *mantleView) rebuildMatches() {
	v.searchMatches = nil
	v.searchIdx = 0
	if v.searchQuery == "" {
		return
	}
	q := strings.ToLower(v.searchQuery)
	lines := strings.Split(v.rendered, "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), q) {
			v.searchMatches = append(v.searchMatches, i)
		}
	}
	// Auto-scroll to first match.
	if len(v.searchMatches) > 0 {
		v.scroll = v.searchMatches[0]
	}
}

// clearSearch resets all search state.
func (v *mantleView) clearSearch() {
	v.searchActive = false
	v.searchQuery = ""
	v.searchMatches = nil
	v.searchIdx = 0
}

func (v mantleView) View() string {
	if v.content != "" {
		return v.viewReader()
	}

	var s string
	s += "  " + StyleTitle.Render("Mantle") + "\n\n"
	for i, name := range mantleSectionNames {
		if mantleSection(i) == v.section {
			s += StyleActiveTab.Render(" [" + name + "] ")
		} else {
			s += StyleInactiveTab.Render("  " + name + "  ")
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
			for i, a := range v.agents {
				if i == v.cursor {
					s += StyleCursor.Render("> "+a.Name) + "\n"
				} else {
					s += "  " + a.Name + "\n"
				}
			}
		}
	case mantleDocs:
		s += "  " + StyleTitle.Render("README.md") + "\n\n  enter to read\n"
	case mantleSettings:
		s += "  " + StyleTitle.Render("work_dirs") + "\n\n"
		if len(v.cfg.WorkDirs) == 0 {
			s += "  (none)\n"
		} else {
			for i, dir := range v.cfg.WorkDirs {
				if i == v.cursor {
					s += StyleCursor.Render("> "+dir) + "\n"
				} else {
					s += "  " + dir + "\n"
				}
			}
		}
		if v.editing {
			s += "\n  new path: " + v.input + "█\n"
		} else {
			s += "\n  a add  d delete\n"
		}
		s += "\n  " + StyleTitle.Render("slug_mode") + "  " + v.cfg.EffectiveSlugMode() + "  (s to cycle)\n"
		wenyanVal := "off"
		if v.cfg.WenyanMode {
			wenyanVal = "on"
		}
		s += "  " + StyleTitle.Render("wenyan_mode") + " " + wenyanVal + "  (w to toggle)\n"
	case mantleSkills:
		if len(v.skills) == 0 {
			s += "  no skills found\n"
		} else {
			for i, name := range v.skills {
				if i == v.cursor {
					s += StyleCursor.Render("> "+name) + "\n"
				} else {
					s += "  " + name + "\n"
				}
			}
		}
	}
	return s
}

func (v mantleView) viewReader() string {
	lines := strings.Split(v.rendered, "\n")
	pageH := v.h - 4 // title line + blank + status line + blank
	if pageH < 5 {
		pageH = 20
	}
	end := v.scroll + pageH
	if end > len(lines) {
		end = len(lines)
	}

	matchSet := make(map[int]bool, len(v.searchMatches))
	for _, idx := range v.searchMatches {
		matchSet[idx] = true
	}
	// Current match gets a distinct indicator.
	currentMatch := -1
	if len(v.searchMatches) > 0 {
		currentMatch = v.searchMatches[v.searchIdx]
	}

	var sb strings.Builder
	sb.WriteString("  " + StyleTitle.Render("Mantle — "+v.title) + "\n\n")
	for i, line := range lines[v.scroll:end] {
		absIdx := v.scroll + i
		if absIdx == currentMatch {
			sb.WriteString(StyleHighlight.Bold(true).Render(line) + "\n")
		} else if matchSet[absIdx] {
			sb.WriteString(StyleHighlight.Render(line) + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}

	// Status line: scroll position + search info.
	status := fmt.Sprintf("line %d/%d", v.scroll+1, len(lines))
	if v.searchQuery != "" {
		if len(v.searchMatches) == 0 {
			status += fmt.Sprintf("  search: %q  (no matches)", v.searchQuery)
		} else {
			status += fmt.Sprintf("  search: %q  %d/%d", v.searchQuery, v.searchIdx+1, len(v.searchMatches))
		}
	}
	if v.searchActive {
		status = "search: " + v.searchQuery + "█"
	}
	sb.WriteString("\n  " + status + "\n")
	return sb.String()
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
