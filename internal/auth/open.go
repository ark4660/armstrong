package auth

import (
	"path/filepath"

	"github.com/ark4660/armstrong/internal/app"
)

// Open loads the launcher's accounts with the OS keyring for secrets. A
// player name from the pre-accounts settings becomes an offline account.
func Open(paths app.Paths, settings *app.Settings) (*Accounts, bool, error) {
	secrets := &KeyringSecrets{Service: app.Name, FilePath: filepath.Join(paths.Root, "secrets.json")}
	accounts, err := OpenAccounts(paths.Root, secrets, NewMicrosoft(app.UserAgent()))
	if err != nil {
		return nil, false, err
	}
	changed := false
	if settings.LegacyOfflineName != "" {
		if len(accounts.List()) == 0 {
			if _, err := accounts.AddOffline(settings.LegacyOfflineName); err != nil {
				return nil, false, err
			}
		}
		settings.LegacyOfflineName = ""
		changed = true
	}
	return accounts, changed, nil
}

// SecretsBackend says where sign-ins are stored, for the UI.
func (a *Accounts) SecretsBackend() string { return a.secrets.Backend() }

// Microsoft is the client accounts sign in with.
func (a *Accounts) Microsoft() *Microsoft { return a.ms }
