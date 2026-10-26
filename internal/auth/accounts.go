package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/zalando/go-keyring"
)

const (
	TypeMicrosoft = "msa"
	TypeOffline   = "offline"
)

// Account is a saved player. Secrets (refresh tokens) are never in here.
type Account struct {
	ID   string `json:"id"` // Minecraft UUID without dashes
	Name string `json:"name"`
	Type string `json:"type"`
}

func (a Account) Label() string {
	if a.Type == TypeMicrosoft {
		return a.Name + " (Microsoft)"
	}
	return a.Name + " (offline)"
}

// Accounts is accounts.json plus the secret store. Safe for concurrent use.
type Accounts struct {
	path    string
	secrets Secrets
	ms      *Microsoft

	mu     sync.Mutex
	list   []Account
	active string
}

type accountsFile struct {
	Accounts []Account `json:"accounts"`
	Active   string    `json:"active,omitempty"`
}

// OpenAccounts loads accounts.json from dir.
func OpenAccounts(dir string, secrets Secrets, ms *Microsoft) (*Accounts, error) {
	a := &Accounts{path: filepath.Join(dir, "accounts.json"), secrets: secrets, ms: ms}
	data, err := os.ReadFile(a.path)
	if errors.Is(err, fs.ErrNotExist) {
		return a, nil
	}
	if err != nil {
		return nil, err
	}
	var f accountsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("accounts.json: %w", err)
	}
	a.list, a.active = f.Accounts, f.Active
	return a, nil
}

// List returns the accounts in the order they were added.
func (a *Accounts) List() []Account {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.list)
}

// Active returns the account Play uses, falling back to the first one.
func (a *Accounts) Active() (Account, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, acc := range a.list {
		if acc.ID == a.active {
			return acc, true
		}
	}
	if len(a.list) > 0 {
		return a.list[0], true
	}
	return Account{}, false
}

func (a *Accounts) SetActive(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.active = id
	return a.saveLocked()
}

// AddOffline adds (or re-selects) an offline account.
func (a *Accounts) AddOffline(name string) (Account, error) {
	if err := ValidateOfflineName(name); err != nil {
		return Account{}, err
	}
	acc := Account{ID: OfflineUUID(name), Name: name, Type: TypeOffline}
	return acc, a.put(acc)
}

// CompleteLogin stores a Microsoft account after a device-code sign-in and
// returns its first session. The refresh token goes to the secret store.
func (a *Accounts) CompleteLogin(ctx context.Context, tok *MSToken) (Account, *Session, error) {
	sess, err := a.ms.MinecraftSession(ctx, tok.AccessToken)
	if err != nil {
		return Account{}, nil, err
	}
	acc := Account{ID: sess.UUID, Name: sess.Name, Type: TypeMicrosoft}
	if err := a.secrets.Set(acc.ID, tok.RefreshToken); err != nil {
		return Account{}, nil, fmt.Errorf("saving sign-in: %w", err)
	}
	return acc, sess, a.put(acc)
}

// Session signs an account in for a launch. Microsoft accounts refresh their
// tokens every time (and save the rotated refresh token).
func (a *Accounts) Session(ctx context.Context, acc Account) (*Session, error) {
	if acc.Type == TypeOffline {
		return &Session{Name: acc.Name, UUID: acc.ID, AccessToken: "0", UserType: "legacy", XUID: "0"}, nil
	}
	refresh, err := a.secrets.Get(acc.ID)
	if errors.Is(err, ErrNoSecret) {
		return nil, ErrReauth
	}
	if err != nil {
		return nil, fmt.Errorf("reading sign-in: %w", err)
	}
	tok, err := a.ms.Refresh(ctx, refresh)
	if err != nil {
		return nil, err
	}
	if tok.RefreshToken != "" {
		if err := a.secrets.Set(acc.ID, tok.RefreshToken); err != nil {
			return nil, fmt.Errorf("saving sign-in: %w", err)
		}
	}
	sess, err := a.ms.MinecraftSession(ctx, tok.AccessToken)
	if err != nil {
		return nil, err
	}
	if sess.Name != acc.Name { // the player changed their name
		acc.Name = sess.Name
		a.put(acc)
	}
	return sess, nil
}

// Remove signs an account out and forgets it.
func (a *Accounts) Remove(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	i := slices.IndexFunc(a.list, func(acc Account) bool { return acc.ID == id })
	if i < 0 {
		return nil
	}
	if a.list[i].Type == TypeMicrosoft {
		if err := a.secrets.Delete(id); err != nil && !errors.Is(err, ErrNoSecret) {
			return fmt.Errorf("removing sign-in: %w", err)
		}
	}
	a.list = slices.Delete(a.list, i, i+1)
	if a.active == id {
		a.active = ""
	}
	return a.saveLocked()
}

// put adds or updates an account and makes it active.
func (a *Accounts) put(acc Account) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if i := slices.IndexFunc(a.list, func(x Account) bool { return x.ID == acc.ID }); i >= 0 {
		a.list[i] = acc
	} else {
		a.list = append(a.list, acc)
	}
	a.active = acc.ID
	return a.saveLocked()
}

func (a *Accounts) saveLocked() error {
	data, err := json.MarshalIndent(accountsFile{Accounts: a.list, Active: a.active}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return err
	}
	tmp := a.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.path)
}

// ErrNoSecret means no refresh token is stored for an account.
var ErrNoSecret = errors.New("no stored sign-in")

// Secrets stores refresh tokens by account id.
type Secrets interface {
	Get(id string) (string, error)
	Set(id, secret string) error
	Delete(id string) error
	// Backend names where secrets end up, for the UI.
	Backend() string
}

// KeyringSecrets uses the OS keyring (Secret Service, Keychain, Credential
// Manager) and falls back to a file readable only by the user when the
// keyring is unavailable or refuses the token (Windows caps secrets at 2560
// bytes). The file is not encrypted: a key kept on the same disk wouldn't
// protect it, so it relies on file permissions, like ~/.ssh does.
type KeyringSecrets struct {
	Service  string
	FilePath string

	mu       sync.Mutex
	usedFile bool
}

func (k *KeyringSecrets) Get(id string) (string, error) {
	s, err := keyring.Get(k.Service, id)
	if err == nil {
		return s, nil
	}
	file, ferr := k.readFile()
	if ferr != nil {
		return "", ferr
	}
	if s, ok := file[id]; ok {
		k.mu.Lock()
		k.usedFile = true
		k.mu.Unlock()
		return s, nil
	}
	return "", ErrNoSecret
}

func (k *KeyringSecrets) Set(id, secret string) error {
	if err := keyring.Set(k.Service, id, secret); err == nil {
		return k.deleteFromFile(id) // don't leave a stale copy behind
	}
	k.mu.Lock()
	k.usedFile = true
	k.mu.Unlock()
	return k.updateFile(func(m map[string]string) { m[id] = secret })
}

func (k *KeyringSecrets) Delete(id string) error {
	err := keyring.Delete(k.Service, id)
	ferr := k.deleteFromFile(id)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) && ferr != nil {
		return err
	}
	return nil
}

func (k *KeyringSecrets) Backend() string {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.usedFile {
		return "file " + k.FilePath + " (no system keyring available)"
	}
	return "system keyring"
}

func (k *KeyringSecrets) readFile() (map[string]string, error) {
	data, err := os.ReadFile(k.FilePath)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", k.FilePath, err)
	}
	return m, nil
}

func (k *KeyringSecrets) updateFile(change func(map[string]string)) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	m, err := k.readFile()
	if err != nil {
		return err
	}
	change(m)
	if len(m) == 0 {
		err := os.Remove(k.FilePath)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(k.FilePath), 0o700); err != nil {
		return err
	}
	tmp := k.FilePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, k.FilePath)
}

func (k *KeyringSecrets) deleteFromFile(id string) error {
	if _, err := os.Stat(k.FilePath); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return k.updateFile(func(m map[string]string) { delete(m, id) })
}
