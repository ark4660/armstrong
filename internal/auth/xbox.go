package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Session is what the game needs to run as a player.
type Session struct {
	Name        string
	UUID        string // without dashes
	AccessToken string
	UserType    string // "msa" or "legacy" (offline)
	XUID        string
	Expires     time.Time
}

type xboxToken struct {
	Token         string `json:"Token"`
	DisplayClaims struct {
		Xui []struct {
			Uhs string `json:"uhs"`
			Xid string `json:"xid"`
		} `json:"xui"`
	} `json:"DisplayClaims"`
}

// xstsErrors explains XSTS refusals (the XErr field) to the player.
var xstsErrors = map[int64]string{
	2148916227: "this Xbox account is banned",
	2148916229: "this account can't play online until a parent allows it in Microsoft Family settings",
	2148916233: "this Microsoft account has no Xbox profile yet; sign in once at xbox.com to create one",
	2148916235: "Xbox Live isn't available in this account's country or region",
	2148916236: "this account needs adult verification on xbox.com (South Korea)",
	2148916237: "this account needs adult verification on xbox.com (South Korea)",
	2148916238: "this is a child account; an adult has to add it to a Microsoft Family first",
}

// MinecraftSession turns a Microsoft access token into a Minecraft session:
// Xbox Live user token → XSTS token → Minecraft token → profile.
func (m *Microsoft) MinecraftSession(ctx context.Context, msAccessToken string) (*Session, error) {
	var xbl xboxToken
	err := m.postJSON(ctx, m.Endpoints.XboxUser, map[string]any{
		"Properties": map[string]any{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  "d=" + msAccessToken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}, &xbl, nil)
	if err != nil {
		return nil, fmt.Errorf("Xbox Live sign-in: %w", err)
	}

	var xsts xboxToken
	err = m.postJSON(ctx, m.Endpoints.XSTS, map[string]any{
		"Properties": map[string]any{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xbl.Token},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}, &xsts, func(status int, body []byte) error {
		var e struct {
			XErr int64 `json:"XErr"`
		}
		if status == http.StatusUnauthorized && json.Unmarshal(body, &e) == nil && e.XErr != 0 {
			if msg, ok := xstsErrors[e.XErr]; ok {
				return errors.New(msg)
			}
			return fmt.Errorf("Xbox Live refused the sign-in (XErr %d)", e.XErr)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Xbox Live: %w", err)
	}
	if len(xsts.DisplayClaims.Xui) == 0 {
		return nil, errors.New("Xbox Live: no user hash in XSTS token")
	}
	uhs := xsts.DisplayClaims.Xui[0].Uhs

	var mc struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	err = m.postJSON(ctx, m.Endpoints.MCLogin, map[string]string{
		"identityToken": "XBL3.0 x=" + uhs + ";" + xsts.Token,
	}, &mc, func(status int, _ []byte) error {
		if status == http.StatusForbidden { // where unapproved apps are rejected
			return ErrAppNotApproved
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Minecraft sign-in: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.Endpoints.MCProfile, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+mc.AccessToken)
	var profile struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	err = m.do(req, &profile, func(status int, _ []byte) error {
		if status == http.StatusNotFound {
			return ErrNoMinecraft
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	xuid := "0"
	if x := xsts.DisplayClaims.Xui[0].Xid; x != "" {
		xuid = x
	}
	return &Session{
		Name:        profile.Name,
		UUID:        profile.ID,
		AccessToken: mc.AccessToken,
		UserType:    "msa",
		XUID:        xuid,
		Expires:     time.Now().Add(time.Duration(mc.ExpiresIn) * time.Second),
	}, nil
}
