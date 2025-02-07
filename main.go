package main

// A simple example demonstrating the use of multiple text input components
// from the Bubbles component library.

import (
	"fmt"
	"os"
  "io"
  "net/http"
  "time"
	"strings"
  "encoding/json"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle         = focusedStyle
	noStyle             = lipgloss.NewStyle()
	helpStyle           = blurredStyle
	cursorModeHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	focusedButton = focusedStyle.Render("[ Check ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Check"))
)

// curl https://api.github.com/users/notTGY/repos
// curl https://api.github.com/repos/notTGY/mojango/commits
const baseRepos = "https://api.github.com/repos"
const baseUsers = "https://api.github.com/users"

type Author struct {
  Email string `json:"email"`
}
type Commit struct {
  Author Author `json:"author"`
}
type CommitDataPiece struct {
  Commit Commit `json:"commit"`
}

type RepoDataPiece struct {
  FullName string `json:"full_name"`
}

type model struct {
	focusIndex int
	inputs     []textinput.Model
	cursorMode cursor.Mode

  isLoading bool
  data []string
  err error
}

type dataMsg struct{ data []string }
type errMsg struct{ err error }
func (e errMsg) Error() string { return e.err.Error() }


func getRepos(user string) (error, []string) {
  data := []string{}
  c := &http.Client{Timeout: 10 * time.Second}
  url := fmt.Sprintf("%s/%s/repos", baseUsers, user)
  res, err := c.Get(url)
  if err != nil {
    return err, data
  }
  defer res.Body.Close()
  body, err := io.ReadAll(res.Body)
  if err != nil {
    return err, data
  }

  var repoData []RepoDataPiece
  err = json.Unmarshal(body, &repoData)
  if err != nil {
    return err, data
  }

  for _, d := range repoData {
    repo := d.FullName
    data = append(data, repo)
  }

  return nil, data
}

func getRepoEmails(fullName string) (error, []string) {
  data := []string{}
  c := &http.Client{Timeout: 10 * time.Second}
  url := fmt.Sprintf("%s/%s/commits", baseRepos, fullName)
  res, err := c.Get(url)
  if err != nil {
    return err, data
  }
  defer res.Body.Close()
  body, err := io.ReadAll(res.Body)
  if err != nil {
    return err, data
  }

  var commitData []CommitDataPiece
  err = json.Unmarshal(body, &commitData)
  if err != nil {
    return err, data
  }

  uniqueEmails := make(map[string]struct{})
  for _, d := range commitData {
    email := d.Commit.Author.Email
    _, exists := uniqueEmails[email]
    if !exists {
      data = append(data, email)
      uniqueEmails[email] = struct{}{}
    }
  }

  return nil, data
}

func checkServer(user string) tea.Cmd {
  return func() tea.Msg {
    err, repos := getRepos(user)
    if err != nil {
      return errMsg{err}
    }

    data := []string{}
    uniqueEmails := make(map[string]struct{})
    for _, repo := range repos {
      err, repoEmails := getRepoEmails(repo)
      if err != nil {
        return errMsg{err}
      }
      for _, email := range repoEmails {
        _, exists := uniqueEmails[email]
        if !exists {
          data = append(data, email)
          uniqueEmails[email] = struct{}{}
        }
      }

    }
    return dataMsg{data}
  }
}

func initialModel() model {
	m := model{
		inputs: make([]textinput.Model, 1),
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 32

		switch i {
      case 0:
        t.Placeholder = "Nickname"
        t.Focus()
        t.PromptStyle = focusedStyle
        t.TextStyle = focusedStyle
		}

		m.inputs[i] = t
	}

	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

    case dataMsg:
      m.data = msg.data
      return m, tea.Quit
    case errMsg:
      m.err = msg
      return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		// Change cursor mode
		case "ctrl+r":
			m.cursorMode++
			if m.cursorMode > cursor.CursorHide {
				m.cursorMode = cursor.CursorBlink
			}
			cmds := make([]tea.Cmd, len(m.inputs))
			for i := range m.inputs {
				cmds[i] = m.inputs[i].Cursor.SetMode(m.cursorMode)
			}
			return m, tea.Batch(cmds...)

		// Set focus to next input
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Did the user press enter while the submit button was focused?
			// If so, exit.
			if s == "enter" && m.focusIndex == len(m.inputs) {
        m.isLoading = true
				return m, checkServer(m.inputs[0].Value())
			}

			// Cycle indexes
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIndex {
					// Set focused state
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = focusedStyle
					m.inputs[i].TextStyle = focusedStyle
					continue
				}
				// Remove focused state
				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = noStyle
				m.inputs[i].TextStyle = noStyle
			}

			return m, tea.Batch(cmds...)
		}
	}

	// Handle character input and blinking
	cmd := m.updateInputs(msg)

	return m, cmd
}

func (m *model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only text inputs with Focus() set will respond, so it's safe to simply
	// update all of them here without any further logic.
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m model) View() string {
  if m.err != nil {
    return fmt.Sprintf("\nWe had some trouble: %v\n\n", m.err)
  }
  if len(m.data) > 0 {
    s := "\n"
    for i, email := range m.data {
      s += fmt.Sprintf("%d.\t%s\n", i+1, email)
    }

    return s + "\n\n"
  }

  if m.isLoading {
    return "Loading..."
  }

	var b strings.Builder

	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			b.WriteRune('\n')
		}
	}

	button := &blurredButton
	if m.focusIndex == len(m.inputs) {
		button = &focusedButton
	}
	fmt.Fprintf(&b, "\n\n%s\n\n", *button)

	b.WriteString(helpStyle.Render("cursor mode is "))
	b.WriteString(cursorModeHelpStyle.Render(m.cursorMode.String()))
	b.WriteString(helpStyle.Render(" (ctrl+r to change style)"))

	return b.String()
}

func main() {
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Printf("could not start program: %s\n", err)
		os.Exit(1)
	}
}
