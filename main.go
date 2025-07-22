package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"
	"github.com/slack-go/slack"
)

type model struct {
	codeInput    textinput.Model
	slackClient  *slack.Client
	statusInput  textinput.Model
	status       string
	emojiInput   textinput.Model
	emoji        string
	userName     string
	errorMessage string
	state        string
}

var (
	clientID     = ""
	clientSecret = ""
	redirectURI  = ""
)

func main() {
	if clientID == "" || clientSecret == "" || redirectURI == "" {
		fmt.Println("Missing build-time secrets. Make sure you injected SLACK_CLIENT_ID, etc.")
		os.Exit(1)
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

	statusInput := textinput.New()
	statusInput.Placeholder = "Input your new status here..."
	statusInput.Width = 100

	emojiInput := textinput.New()
	emojiInput.Placeholder = "Input your new status emoji here..."
	emojiInput.Width = 100

	return model{
		codeInput:   codeInput,
		statusInput: statusInput,
		emojiInput:  emojiInput,
		state:       "initial",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case "initial":
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q", "ctrl+c":
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

	case "showCodeInput":
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter":
				code := strings.TrimSpace(m.codeInput.Value())
				if code == "" {
					m.errorMessage = "Please enter the code!\n\n"
					return m, nil
				}
				token, err := exchangeCodeForToken(code)
				if err != nil {
					m.errorMessage = "Authorization failed\n\n"
					return m, nil
				}
				api := slack.New(token)
				m.slackClient = api
				authTest, err := api.AuthTest()
				if err != nil {
					m.errorMessage = "Could not authenticate with Slack\n\n"
					return m, nil
				}
				user, err := api.GetUserInfo(authTest.UserID)
				if err != nil {
					m.errorMessage = "Could not authenticate with Slack\n\n"
					return m, nil
				}
				name := user.Profile.DisplayName
				if name == "" {
					name = user.Profile.FirstName
				}
				m.userName = name
				m.state = "showStatusInput"
				m.statusInput.Focus()
				return m, textinput.Blink
			}
		}
		var cmd tea.Cmd
		m.codeInput, cmd = m.codeInput.Update(msg)
		return m, cmd

	case "showStatusInput":
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter":
				m.status = m.statusInput.Value()
				m.state = "showEmojiInput"
				m.emojiInput.Focus()
				return m, textinput.Blink
			}
		}
		var cmd tea.Cmd
		m.statusInput, cmd = m.statusInput.Update(msg)
		return m, cmd

	case "showEmojiInput":
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter":
				m.emoji = m.emojiInput.Value()
				if err := m.slackClient.SetUserCustomStatus(m.status, m.emoji, 0); err != nil {
					m.errorMessage = "Error updating your slack status"
					return m, nil
				}
				m.state = "end"
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.emojiInput, cmd = m.emojiInput.Update(msg)
		return m, cmd
	case "end":
		return m, tea.Quit
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
	case "showStatusInput":
		s += "Welcome " + m.userName + "!" + "\n\n"
		s += m.statusInput.View() + "\n\n"
	case "showEmojiInput":
		s += "Welcome " + m.userName + "!" + "\n\n"
		s += m.statusInput.View() + "\n\n"
		s += m.emojiInput.View() + "\n\n"
	case "end":
		if m.errorMessage != "" {
			s += "We couldn't update your status 😔\n\nHave a great day\n\n"
		} else {
			s += "✅ SUCCESS ✅\n\nEnjoy your new status\n\n"
		}
	}
	s += m.errorMessage
	s += "Press q to quit\n"
	return s
}

func getOauthUrl() string {
	params := url.Values{}
	params.Add("client_id", clientID)
	params.Add("user_scope", "users.profile:write,users:read")
	params.Add("redirect_uri", redirectURI)

	return "https://slack.com/oauth/v2/authorize?" + params.Encode()
}

func exchangeCodeForToken(code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	resp, err := http.PostForm("https://slack.com/api/oauth.v2.access", data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var result struct {
		OK         bool   `json:"ok"`
		Error      string `json:"error"`
		AuthedUser struct {
			AccessToken string `json:"access_token"`
		} `json:"authed_user"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if !result.OK {
		return "", fmt.Errorf("slack error: %s", result.Error)
	}

	return result.AuthedUser.AccessToken, nil
}
