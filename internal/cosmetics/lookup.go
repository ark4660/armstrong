package cosmetics

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ark4660/armstrong/internal/download"
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)

// profileURL is Mojang's name → UUID lookup (a var so tests can point it elsewhere).
var profileURL = "https://api.mojang.com/users/profiles/minecraft/"

// Resolve turns a player name, or a UUID with or without dashes, into the
// UUID (no dashes) and current name of a Minecraft account.
func Resolve(ctx context.Context, dl *download.Client, player string) (uuid, name string, err error) {
	if u := strings.ToLower(strings.ReplaceAll(player, "-", "")); uuidPattern.MatchString(u) {
		return u, "", nil
	}
	if !namePattern.MatchString(player) {
		return "", "", fmt.Errorf("%q isn't a Minecraft name or UUID", player)
	}
	var p struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := dl.GetJSON(ctx, profileURL+url.PathEscape(player), &p); err != nil {
		return "", "", fmt.Errorf("no Minecraft account named %s: %w", player, err)
	}
	if !uuidPattern.MatchString(p.ID) {
		return "", "", fmt.Errorf("Mojang returned an odd id %q for %s", p.ID, player)
	}
	return p.ID, p.Name, nil
}
