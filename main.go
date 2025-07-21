package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/pkg/browser"
)

type model struct {
	codeInput    textinput.Model
	code         string
	token        string
	statusInput  textinput.Model
	status       string
	userName     string
	errorMessage string
	state        string
}

var err error = godotenv.Load()

func main() {
	if err != nil {
		fmt.Println("Couldn't load env variables")
	}

	program := tea.NewProgram(initialModel())
	if _, err := program.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func initialModel() model {
	codeInput := textinput.New()
	codeInput.Placeholder = "Paste your authorization code here..."
	codeInput.CharLimit = 100
	codeInput.Width = 100

	return model{
		codeInput: codeInput,
		state:     "initial",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.state == "initial" {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "enter":
				err := browser.OpenURL(getOauthUrl())
				if err != nil {
					err := exec.Command("wslview", getOauthUrl()).Start()
					if err != nil {
						m.errorMessage = "Couldn't open the browser\n\n" + "Open this URL to authorize:" + getOauthUrl() + "\n\n"
						fmt.Println()
					}
				}
				m.state = "showCodeInput"
				m.codeInput.Focus()
				return m, textinput.Blink
			}
		}

	} else if m.state == "showCodeInput" {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "enter":
				code := m.codeInput.Value()
				if code == "" {
					m.errorMessage = "Please enter the code!\n\n"
					return m, nil
				}
				m.code = code
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.codeInput, cmd = m.codeInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	s := "\n"

	switch m.state {
	case "initial":
		s += "Press enter to sign in with Slack\n\n"
	case "showCodeInput":
		s += m.codeInput.View() + "\n\n"
	}
	s += m.errorMessage
	s += "Press q to quit\n"
	return s
}

func getOauthUrl() string {
	clientID := os.Getenv("SLACK_CLIENT_ID")
	redirectURI := os.Getenv("SLACK_REDIRECT_URI")

	params := url.Values{}
	params.Add("client_id", clientID)
	params.Add("user_scope", "users.profile:write")
	params.Add("redirect_uri", redirectURI)

	return "https://slack.com/oauth/v2/authorize?" + params.Encode()
}
