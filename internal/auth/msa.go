package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClientID is Armstrong's Azure app, approved by Mojang for the Minecraft
// API. It's a public client: there is no secret, and none must ever ship.
const ClientID = "f6945deb-3d58-44a3-bf89-14286402ba29"

const scope = "XboxLive.signin offline_access"

// Endpoints are the services in the sign-in chain; tests point them at a fake.
type Endpoints struct {
	Microsoft string // OAuth base: .../consumers/oauth2/v2.0
	XboxUser  string
	XSTS      string
	MCLogin   string
	MCProfile string
}

var DefaultEndpoints = Endpoints{
	Microsoft: "https://login.microsoftonline.com/consumers/oauth2/v2.0",
	XboxUser:  "https://user.auth.xboxlive.com/user/authenticate",
	XSTS:      "https://xsts.auth.xboxlive.com/xsts/authorize",
	MCLogin:   "https://api.minecraftservices.com/authentication/login_with_xbox",
	MCProfile: "https://api.minecraftservices.com/minecraft/profile",
}

// Microsoft talks to Microsoft, Xbox Live and Minecraft services.
type Microsoft struct {
	HTTP      *http.Client
	UserAgent string
	Endpoints Endpoints
}

func NewMicrosoft(userAgent string) *Microsoft {
	return &Microsoft{HTTP: &http.Client{Timeout: 30 * time.Second}, UserAgent: userAgent, Endpoints: DefaultEndpoints}
}

// DeviceCode is what the player needs to sign in on another device or browser.
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

// MSToken is a Microsoft OAuth token.
type MSToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type oauthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *oauthError) Error() string {
	if i := strings.Index(e.Description, "\r\n"); i > 0 {
		return e.Code + ": " + e.Description[:i]
	}
	return e.Code + ": " + e.Description
}

// Errors the UI explains to the player.
var (
	ErrLoginDeclined = errors.New("sign-in was declined")
	ErrLoginExpired  = errors.New("the sign-in code expired; start again")
	ErrReauth        = errors.New("your Microsoft sign-in has expired; please sign in again")
	ErrNoMinecraft   = errors.New("this Microsoft account doesn't own Minecraft: Java Edition")
	// ErrAppNotApproved means Mojang hasn't approved ClientID for the
	// Minecraft API (never submitted, still in review, or declined).
	ErrAppNotApproved = errors.New("the launcher's Azure app isn't approved for the Minecraft API (submit the client ID at https://aka.ms/mce-reviewappid)")
)

// StartDeviceLogin asks Microsoft for a code the player enters at VerificationURI.
func (m *Microsoft) StartDeviceLogin(ctx context.Context) (*DeviceCode, error) {
	var dc DeviceCode
	form := url.Values{"client_id": {ClientID}, "scope": {scope}}
	if err := m.postForm(ctx, m.Endpoints.Microsoft+"/devicecode", form, &dc); err != nil {
		return nil, fmt.Errorf("starting sign-in: %w", err)
	}
	if dc.Interval <= 0 {
		dc.Interval = 5
	}
	return &dc, nil
}

// WaitForLogin polls until the player finished signing in, declined, or the
// code expired.
func (m *Microsoft) WaitForLogin(ctx context.Context, dc *DeviceCode) (*MSToken, error) {
	interval := time.Duration(dc.Interval) * time.Second
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	form := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {ClientID},
		"device_code": {dc.DeviceCode},
	}
	for {
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		var tok MSToken
		err := m.postForm(ctx, m.Endpoints.Microsoft+"/token", form, &tok)
		var oe *oauthError
		switch {
		case err == nil:
			return &tok, nil
		case errors.As(err, &oe) && oe.Code == "authorization_pending":
		case errors.As(err, &oe) && oe.Code == "slow_down":
			interval += 5 * time.Second
		case errors.As(err, &oe) && oe.Code == "authorization_declined":
			return nil, ErrLoginDeclined
		case errors.As(err, &oe) && oe.Code == "expired_token":
			return nil, ErrLoginExpired
		default:
			return nil, fmt.Errorf("signing in: %w", err)
		}
		if time.Now().After(deadline) {
			return nil, ErrLoginExpired
		}
	}
}

// Refresh trades a refresh token for a new token pair. Microsoft rotates
// refresh tokens, so the returned one must be saved.
func (m *Microsoft) Refresh(ctx context.Context, refreshToken string) (*MSToken, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {ClientID},
		"refresh_token": {refreshToken},
		"scope":         {scope},
	}
	var tok MSToken
	err := m.postForm(ctx, m.Endpoints.Microsoft+"/token", form, &tok)
	var oe *oauthError
	if errors.As(err, &oe) && (oe.Code == "invalid_grant" || oe.Code == "interaction_required") {
		return nil, ErrReauth
	}
	if err != nil {
		return nil, fmt.Errorf("refreshing sign-in: %w", err)
	}
	return &tok, nil
}

func (m *Microsoft) postForm(ctx context.Context, u string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return m.do(req, out, func(status int, body []byte) error {
		var oe oauthError
		if json.Unmarshal(body, &oe) == nil && oe.Code != "" {
			return &oe
		}
		return nil
	})
}

func (m *Microsoft) postJSON(ctx context.Context, u string, in, out any, onError func(int, []byte) error) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return m.do(req, out, onError)
}

// do sends req and decodes a 2xx JSON body into out. For other statuses,
// onError may turn the body into a specific error.
func (m *Microsoft) do(req *http.Request, out any, onError func(int, []byte) error) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", m.UserAgent)
	resp, err := m.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if onError != nil {
			if err := onError(resp.StatusCode, body); err != nil {
				return err
			}
		}
		return fmt.Errorf("%s: HTTP %s", req.URL.Host, resp.Status)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s: %w", req.URL.Host, err)
	}
	return nil
}
