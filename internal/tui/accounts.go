package tui

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ark4660/armstrong/internal/auth"
)

// Messages from the Microsoft sign-in goroutine.
type (
	loginCodeMsg   struct{ code *auth.DeviceCode }
	loginStepMsg   struct{ step string }
	loginDoneMsg   struct{ acc auth.Account }
	loginFailedMsg struct{ err error }
)

// login is an in-progress Microsoft device-code sign-in.
type login struct {
	active bool
	code   *auth.DeviceCode
	step   string
	cancel context.CancelFunc
}

func (l *login) keys() []keyHelp {
	if l.code == nil {
		return []keyHelp{{"esc", "cancel"}}
	}
	return []keyHelp{{"o", "open browser"}, {"y", "copy code"}, {"esc", "cancel"}}
}

func (m *model) reloadAccounts() {
	list := m.accounts.List()
	labels := make([]string, len(list))
	cursor := 0
	active, _ := m.accounts.Active()
	for i, acc := range list {
		labels[i] = acc.Label()
		if acc.ID == active.ID {
			cursor = i
		}
	}
	m.accList.setItems(labels)
	m.accList.selectIndex(cursor)
}

func (m *model) cursorAccount() (auth.Account, bool) {
	list := m.accounts.List()
	if i := m.accList.selected(); i >= 0 && i < len(list) {
		return list[i], true
	}
	return auth.Account{}, false
}

// accountAdded refreshes the list and, on first run, moves on to creating
// the first instance.
func (m model) accountAdded() (tea.Model, tea.Cmd) {
	m.reloadAccounts()
	if len(m.instances) == 0 {
		return m.openWizard()
	}
	m.screen = screenHome
	return m, nil
}

func (m model) handleAccountsKey(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "m":
		return m.startLogin()
	case "o":
		m.editInput(inputOffline, "", "Steve", 16)
	case "enter":
		if acc, ok := m.cursorAccount(); ok {
			if err := m.accounts.SetActive(acc.ID); err != nil {
				m.notice = "Couldn't save accounts: " + err.Error()
			}
			m.reloadAccounts()
			m.screen = screenHome
		}
	case "d":
		if _, ok := m.cursorAccount(); ok {
			m.confirmRemove = true
		}
	default:
		m.accList.handleKey(k)
	}
	return m, nil
}

func (m model) removeAccount() (tea.Model, tea.Cmd) {
	acc, ok := m.cursorAccount()
	if !ok {
		return m, nil
	}
	if err := m.accounts.Remove(acc.ID); err != nil {
		m.notice = err.Error()
	}
	m.reloadAccounts()
	return m, nil
}

// startLogin runs the device-code flow in the background: get a code, wait
// for the player to enter it in a browser, then sign in to Xbox and Minecraft.
func (m model) startLogin() (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.login = login{active: true, step: "Asking Microsoft for a sign-in code", cancel: cancel}
	m.notice = ""
	accounts, send := m.accounts, m.send.fn
	ms := accounts.Microsoft()
	go func() {
		dc, err := ms.StartDeviceLogin(ctx)
		if err != nil {
			send(loginFailedMsg{err})
			return
		}
		send(loginCodeMsg{dc})
		tok, err := ms.WaitForLogin(ctx, dc)
		if err != nil {
			send(loginFailedMsg{err})
			return
		}
		send(loginStepMsg{"Signing in to Xbox Live and Minecraft"})
		acc, _, err := accounts.CompleteLogin(ctx, tok)
		if err != nil {
			send(loginFailedMsg{err})
			return
		}
		send(loginDoneMsg{acc})
	}()
	return m, m.spin.Tick
}

func (m model) handleLoginMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.login.active {
		return m, nil // cancelled; late results are dropped
	}
	switch msg := msg.(type) {
	case loginCodeMsg:
		m.login.code = msg.code
		m.login.step = "Waiting for you to sign in"
	case loginStepMsg:
		m.login.step = msg.step
	case loginFailedMsg:
		m.login = login{}
		if !errors.Is(msg.err, context.Canceled) {
			m.notice = "Sign-in failed: " + msg.err.Error()
		}
	case loginDoneMsg:
		m.login = login{}
		m.notice = ""
		return m.accountAdded()
	}
	return m, nil
}

func (m model) handleLoginKey(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "esc":
		if m.login.cancel != nil {
			m.login.cancel()
		}
		m.login = login{}
	case "o":
		if m.login.code != nil {
			if err := openURL(m.login.code.VerificationURI); err != nil {
				m.notice = "Couldn't open a browser: " + err.Error()
			}
		}
	case "y":
		if m.login.code != nil {
			return m, tea.SetClipboard(m.login.code.UserCode)
		}
	}
	return m, nil
}

func (m model) accountsView() string {
	s := m.styles
	if m.login.active {
		return m.loginView()
	}
	lines := []string{""}
	list := m.accounts.List()
	if len(list) == 0 {
		lines = append(lines,
			"  "+s.title.Render("Add an account to play"),
			"",
			"  "+s.key.Render("m")+"  Sign in with Microsoft "+s.muted.Render("(needed for online servers and Realms)"),
			"  "+s.key.Render("o")+"  Play offline "+s.muted.Render("(singleplayer and offline-mode servers)"),
		)
	}
	active, _ := m.accounts.Active()
	lines = append(lines, m.accList.view(func(i int, cursor bool) string {
		acc := list[i]
		pointer, mark := "  ", "  "
		if cursor {
			pointer = s.key.Render("› ")
		}
		if acc.ID == active.ID {
			mark = s.accent.Render("▶ ")
		}
		name := acc.Name
		if cursor {
			name = s.value.Bold(true).Render(name)
		}
		kind := s.muted.Render("  offline")
		if acc.Type == auth.TypeMicrosoft {
			kind = s.ok.Render("  Microsoft")
		}
		return pointer + mark + name + kind
	})...)
	if m.inputMode == inputOffline {
		lines = append(lines, "", "  "+s.label.Render("Offline name")+s.accent.Render("› ")+m.input.View())
	}
	if len(list) > 0 {
		lines = append(lines, "", "  "+s.muted.Render("Sign-ins are kept in your "+m.accounts.SecretsBackend()+"."))
	}
	if m.notice != "" {
		lines = append(lines, "", "  "+s.err.Width(max(m.width-4, 20)).Render(m.notice))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m model) loginView() string {
	s := m.styles
	lines := []string{"", "  " + s.title.Render("Sign in with Microsoft"), ""}
	if c := m.login.code; c != nil {
		code := lipgloss.NewStyle().Bold(true).Foreground(s.accent.GetForeground()).
			Border(lipgloss.RoundedBorder()).BorderForeground(s.accent.GetForeground()).
			Padding(0, 3).Render(spaced(c.UserCode))
		lines = append(lines,
			"  1. Open "+s.key.Render(c.VerificationURI)+s.muted.Render("  (press o)"),
			"  2. Enter this code "+s.muted.Render("(press y to copy it)")+":",
			"",
			lipgloss.NewStyle().MarginLeft(5).Render(code),
			"",
		)
	}
	lines = append(lines, "  "+m.spin.View()+" "+m.login.step+"…")
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// spaced puts a space between characters so a code reads easily: "A B C D".
func spaced(code string) string {
	return strings.Join(strings.Split(code, ""), " ")
}

// openURL opens a link in the default browser without waiting for it.
func openURL(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap it
	return nil
}
