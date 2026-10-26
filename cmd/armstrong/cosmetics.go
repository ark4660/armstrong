package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/cosmetics"
	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/update"
)

const cosmeticsUsage = `usage: cosmetics COMMAND (run in the repository; edits cosmetics/ and signs it)
  give PLAYER [-cape ID|-] [-badge ID|-]   give a player a cape and/or badge (- takes it away)
  take PLAYER                              take everything away from a player
  add-cape ID FILE.png [-name NAME]        add or replace a cape (64x32, or 128x64 etc. for HD)
  add-badge ID -symbol S -color #RRGGBB [-name NAME]
  list                                     show capes, badges and players
  sign                                     re-sign after editing cosmetics.json by hand
  keygen                                   make the cosmetics signing key (once)
Then commit and push cosmetics/: players see the change within ~15 minutes.
The key is read from $ARMSTRONG_COSMETICS_KEY or ~/.config/armstrong-release/cosmetics-key.`

// cosmeticsKeyPath is where the cosmetics signing key lives. It's separate
// from the release key, so it can never sign a launcher update.
func cosmeticsKeyPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "armstrong-release", "cosmetics-key"), nil
}

func cosmeticsKey() (string, error) {
	if k := os.Getenv("ARMSTRONG_COSMETICS_KEY"); k != "" {
		return k, nil
	}
	path, err := cosmeticsKeyPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cosmetics key: %w (make one with `cosmetics keygen`)", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func cmdCosmetics(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New(cosmeticsUsage)
	}
	fs := flag.NewFlagSet("cosmetics "+args[0], flag.ExitOnError)
	dir := fs.String("dir", "cosmetics", "the cosmetics folder")
	cape := fs.String("cape", "", "cape id, or - to take it away")
	badge := fs.String("badge", "", "badge id, or - to take it away")
	name := fs.String("name", "", "display name")
	symbol := fs.String("symbol", "", "badge symbol, e.g. ◆")
	color := fs.String("color", "", "badge colour, #RRGGBB")
	// Flags may come after the positional arguments: `give Notch -cape x`.
	var pos []string
	rest := args[1:]
	for len(rest) > 0 {
		fs.Parse(rest)
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		rest = fs.Args()[1:]
	}

	if args[0] == "keygen" {
		path, err := cosmeticsKeyPath()
		if err != nil {
			return err
		}
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing to overwrite %s", path)
		}
		seed, pub, err := update.NewKey()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(seed+"\n"), 0o600); err != nil {
			return err
		}
		fmt.Printf("Cosmetics key written to %s (keep it secret).\nPublic key for the client mod (cosmetics/CosmeticsKey.java):\n%s\n", path, pub)
		return nil
	}

	idx, err := cosmetics.Load(*dir)
	if err != nil {
		return err
	}
	save := func() error {
		key, err := cosmeticsKey()
		if err != nil {
			return err
		}
		if err := idx.Save(*dir, key); err != nil {
			return err
		}
		fmt.Printf("Signed %s. Commit and push %s/ to publish.\n", filepath.Join(*dir, cosmetics.IndexFile), *dir)
		return nil
	}

	switch args[0] {
	case "give", "take":
		if len(pos) != 1 {
			return errors.New(cosmeticsUsage)
		}
		dl := download.New(app.UserAgent())
		uuid, player, err := cosmetics.Resolve(ctx, dl, pos[0])
		if err != nil {
			return err
		}
		if player == "" {
			player = idx.Players[uuid].Name // given a UUID: keep the name we had
		}
		if args[0] == "take" {
			*cape, *badge = "-", "-"
		} else if *cape == "" && *badge == "" {
			return errors.New("give what? -cape ID and/or -badge ID")
		}
		if err := idx.Give(uuid, player, *cape, *badge); err != nil {
			return err
		}
		p := idx.Players[uuid]
		fmt.Printf("%s (%s): cape %s, badge %s\n", player, uuid, orNone(p.Cape), orNone(p.Badge))
		return save()
	case "add-cape":
		if len(pos) != 2 {
			return errors.New(cosmeticsUsage)
		}
		n := *name
		if n == "" {
			n = pos[0]
		}
		if err := idx.AddCape(*dir, pos[0], n, pos[1]); err != nil {
			return err
		}
		return save()
	case "add-badge":
		if len(pos) != 1 || *symbol == "" || *color == "" {
			return errors.New(cosmeticsUsage)
		}
		n := *name
		if n == "" {
			n = pos[0]
		}
		idx.Badges[pos[0]] = cosmetics.Badge{Name: n, Symbol: *symbol, Color: *color}
		return save()
	case "sign":
		return save()
	case "list":
		printCosmetics(idx)
		return idx.Validate(*dir)
	}
	return errors.New(cosmeticsUsage)
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func printCosmetics(idx *cosmetics.Index) {
	fmt.Println("Capes:")
	for _, id := range sortedKeys(idx.Capes) {
		fmt.Printf("  %-16s %s\n", id, idx.Capes[id].Name)
	}
	fmt.Println("Badges:")
	for _, id := range sortedKeys(idx.Badges) {
		b := idx.Badges[id]
		fmt.Printf("  %-16s %s %s %s\n", id, b.Symbol, b.Color, b.Name)
	}
	fmt.Println("Players:")
	ids := sortedKeys(idx.Players)
	sort.Slice(ids, func(a, b int) bool {
		return strings.ToLower(idx.Players[ids[a]].Name) < strings.ToLower(idx.Players[ids[b]].Name)
	})
	for _, uuid := range ids {
		p := idx.Players[uuid]
		fmt.Printf("  %-16s cape %-12s badge %-10s %s\n", p.Name, orNone(p.Cape), orNone(p.Badge), uuid)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
