package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zalando/go-keyring"
)

// fakeServices imitates Microsoft, Xbox Live and Minecraft services.
type fakeServices struct {
	pending     atomic.Int32 // token polls answered with authorization_pending
	declined    bool
	xerr        int64
	noMinecraft bool
	unapproved  bool
	refreshes   atomic.Int32
}

func (f *fakeServices) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("/ms/devicecode", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("client_id") != ClientID || r.Form.Get("scope") != scope {
			t.Errorf("devicecode form = %v", r.Form)
		}
		writeJSON(w, 200, map[string]any{"device_code": "dev", "user_code": "ABCD1234", "verification_uri": "https://www.microsoft.com/link", "expires_in": 900, "interval": 5})
	})
	mux.HandleFunc("/ms/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.Form.Get("grant_type") {
		case "refresh_token":
			n := f.refreshes.Add(1)
			if r.Form.Get("refresh_token") == "revoked" {
				writeJSON(w, 400, map[string]string{"error": "invalid_grant", "error_description": "AADSTS70000: revoked\r\nTrace ID: x"})
				return
			}
			writeJSON(w, 200, map[string]any{"access_token": "ms-access", "refresh_token": "rotated-" + string(rune('0'+n)), "expires_in": 3600})
		default:
			if f.declined {
				writeJSON(w, 400, map[string]string{"error": "authorization_declined"})
				return
			}
			if f.pending.Add(-1) >= 0 {
				writeJSON(w, 400, map[string]string{"error": "authorization_pending"})
				return
			}
			writeJSON(w, 200, map[string]any{"access_token": "ms-access", "refresh_token": "refresh-1", "expires_in": 3600})
		}
	})
	mux.HandleFunc("/xbl", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Properties struct{ RpsTicket string }
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Properties.RpsTicket != "d=ms-access" {
			t.Errorf("RpsTicket = %q", body.Properties.RpsTicket)
		}
		writeJSON(w, 200, map[string]any{"Token": "xbl-token", "DisplayClaims": map[string]any{"xui": []map[string]string{{"uhs": "hash"}}}})
	})
	mux.HandleFunc("/xsts", func(w http.ResponseWriter, r *http.Request) {
		if f.xerr != 0 {
			writeJSON(w, 401, map[string]any{"XErr": f.xerr})
			return
		}
		writeJSON(w, 200, map[string]any{"Token": "xsts-token", "DisplayClaims": map[string]any{"xui": []map[string]string{{"uhs": "hash"}}}})
	})
	mux.HandleFunc("/mc/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ IdentityToken string }
		json.NewDecoder(r.Body).Decode(&body)
		if body.IdentityToken != "XBL3.0 x=hash;xsts-token" {
			t.Errorf("identityToken = %q", body.IdentityToken)
		}
		if f.unapproved {
			writeJSON(w, 403, map[string]string{"path": "/authentication/login_with_xbox", "errorMessage": "Invalid app registration"})
			return
		}
		writeJSON(w, 200, map[string]any{"access_token": "mc-token", "expires_in": 86400})
	})
	mux.HandleFunc("/mc/profile", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mc-token" {
			t.Errorf("profile auth = %q", r.Header.Get("Authorization"))
		}
		if f.noMinecraft {
			writeJSON(w, 404, map[string]string{"error": "NOT_FOUND"})
			return
		}
		writeJSON(w, 200, map[string]string{"id": "0123456789abcdef0123456789abcdef", "name": "Steve"})
	})
	return mux
}

func newFake(t *testing.T, f *fakeServices) *Microsoft {
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	ms := NewMicrosoft("test")
	ms.Endpoints = Endpoints{
		Microsoft: srv.URL + "/ms", XboxUser: srv.URL + "/xbl", XSTS: srv.URL + "/xsts",
		MCLogin: srv.URL + "/mc/login", MCProfile: srv.URL + "/mc/profile",
	}
	return ms
}

func TestDeviceLoginChain(t *testing.T) {
	keyring.MockInit()
	f := &fakeServices{}
	f.pending.Store(2)
	ms := newFake(t, f)
	ctx := context.Background()

	dc, err := ms.StartDeviceLogin(ctx)
	if err != nil || dc.UserCode != "ABCD1234" {
		t.Fatalf("StartDeviceLogin = %+v, %v", dc, err)
	}
	dc.Interval = 0 // don't make the test wait
	tok, err := ms.WaitForLogin(ctx, dc)
	if err != nil || tok.RefreshToken != "refresh-1" {
		t.Fatalf("WaitForLogin = %+v, %v", tok, err)
	}

	secrets := &KeyringSecrets{Service: "armstrong-test", FilePath: filepath.Join(t.TempDir(), "secrets.json")}
	accounts, err := OpenAccounts(t.TempDir(), secrets, ms)
	if err != nil {
		t.Fatal(err)
	}
	acc, sess, err := accounts.CompleteLogin(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Name != "Steve" || acc.Type != TypeMicrosoft || sess.AccessToken != "mc-token" || sess.UserType != "msa" {
		t.Errorf("account %+v, session %+v", acc, sess)
	}
	if got, _ := secrets.Get(acc.ID); got != "refresh-1" {
		t.Errorf("stored refresh token = %q", got)
	}

	// A launch refreshes and saves the rotated refresh token.
	if _, err := accounts.Session(ctx, acc); err != nil {
		t.Fatal(err)
	}
	if got, _ := secrets.Get(acc.ID); got != "rotated-1" {
		t.Errorf("after refresh, stored token = %q", got)
	}
	if active, _ := accounts.Active(); active.ID != acc.ID {
		t.Errorf("active = %+v", active)
	}

	// A revoked refresh token asks the player to sign in again.
	secrets.Set(acc.ID, "revoked")
	if _, err := accounts.Session(ctx, acc); !errors.Is(err, ErrReauth) {
		t.Errorf("revoked token: err = %v, want ErrReauth", err)
	}

	if err := accounts.Remove(acc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := secrets.Get(acc.ID); !errors.Is(err, ErrNoSecret) {
		t.Errorf("secret should be gone after Remove: %v", err)
	}
}

func TestLoginErrors(t *testing.T) {
	ctx := context.Background()

	ms := newFake(t, &fakeServices{declined: true})
	if _, err := ms.WaitForLogin(ctx, &DeviceCode{DeviceCode: "dev", ExpiresIn: 60}); !errors.Is(err, ErrLoginDeclined) {
		t.Errorf("declined: %v", err)
	}

	ms = newFake(t, &fakeServices{xerr: 2148916233})
	if _, err := ms.MinecraftSession(ctx, "ms-access"); err == nil || !strings.Contains(err.Error(), "no Xbox profile") {
		t.Errorf("XErr: %v", err)
	}

	ms = newFake(t, &fakeServices{unapproved: true})
	if _, err := ms.MinecraftSession(ctx, "ms-access"); !errors.Is(err, ErrAppNotApproved) {
		t.Errorf("unapproved app: %v", err)
	}

	ms = newFake(t, &fakeServices{noMinecraft: true})
	if _, err := ms.MinecraftSession(ctx, "ms-access"); !errors.Is(err, ErrNoMinecraft) {
		t.Errorf("no Minecraft: %v", err)
	}
}

func TestSecretsFallBackToFile(t *testing.T) {
	keyring.MockInitWithError(errors.New("no secret service"))
	defer keyring.MockInit()

	path := filepath.Join(t.TempDir(), "secrets.json")
	s := &KeyringSecrets{Service: "armstrong-test", FilePath: path}
	if err := s.Set("id1", "token"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get("id1"); err != nil || got != "token" {
		t.Errorf("Get = %q, %v", got, err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("secrets file mode = %v, %v", st.Mode(), err)
	}
	if !strings.HasPrefix(s.Backend(), "file ") {
		t.Errorf("Backend = %q", s.Backend())
	}
	if err := s.Delete("id1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("empty secrets file should be removed")
	}
}

func TestOfflineAccounts(t *testing.T) {
	accounts, err := OpenAccounts(t.TempDir(), &KeyringSecrets{Service: "armstrong-test", FilePath: filepath.Join(t.TempDir(), "s.json")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.AddOffline("ab"); err == nil {
		t.Error("short name should be rejected")
	}
	acc, err := accounts.AddOffline("Notch")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := accounts.Session(context.Background(), acc)
	if err != nil || sess.UUID != OfflineUUID("Notch") || sess.UserType != "legacy" {
		t.Errorf("offline session = %+v, %v", sess, err)
	}
}

// TestRealKeyring talks to this machine's keyring when ARMSTRONG_TEST_KEYRING=1.
func TestRealKeyring(t *testing.T) {
	if os.Getenv("ARMSTRONG_TEST_KEYRING") != "1" {
		t.Skip("ARMSTRONG_TEST_KEYRING not set")
	}
	const service, user = "armstrong-keyring-test", "probe"
	if err := keyring.Set(service, user, strings.Repeat("x", 2000)); err != nil {
		t.Fatalf("Set: %v", err)
	}
	defer keyring.Delete(service, user)
	got, err := keyring.Get(service, user)
	if err != nil || len(got) != 2000 {
		t.Fatalf("Get: %d bytes, %v", len(got), err)
	}
}
