// Command armstrong is the Armstrong launcher: a terminal launcher for the
// Armstrong Minecraft client.
//
// Run without arguments for the TUI. Headless subcommands drive the same core
// for scripting and testing:
//
//	armstrong install   [-mc 26.3] [-loader fabric|vanilla] [-loader-version X]
//	armstrong instances
//	armstrong launch    -instance ID -offline NAME [-mem MB] [-server host:port]
//	armstrong boot-test [-instance ID | -mc 26.3 -loader fabric] [-timeout 3m]
//	armstrong client resolve [-mc 26.3] [-o FILE]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ark4660/armstrong/internal/app"
	"github.com/ark4660/armstrong/internal/auth"
	"github.com/ark4660/armstrong/internal/client"
	"github.com/ark4660/armstrong/internal/download"
	"github.com/ark4660/armstrong/internal/game"
	"github.com/ark4660/armstrong/internal/instance"
	"github.com/ark4660/armstrong/internal/modrinth"
	"github.com/ark4660/armstrong/internal/mods"
	"github.com/ark4660/armstrong/internal/tui"
	"github.com/ark4660/armstrong/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		if err := runTUI(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// These don't touch the data folder (release runs in CI on a bare machine).
	switch os.Args[1] {
	case "version":
		fmt.Println(app.Name, app.Version, update.Platform())
		return
	case "release":
		if err := cmdRelease(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	case "cosmetics":
		if err := cmdCosmetics(ctx, os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	paths, err := app.DefaultPaths()
	var accounts *auth.Accounts
	if err == nil {
		accounts, err = setup(paths)
	}
	if err == nil {
		switch os.Args[1] {
		case "update":
			err = cmdUpdate(ctx, paths, os.Args[2:])
		case "accounts":
			err = cmdAccounts(accounts)
		case "login":
			err = cmdLogin(ctx, accounts)
		case "install":
			err = cmdInstall(ctx, paths, os.Args[2:])
		case "instances":
			err = cmdInstances(paths)
		case "launch":
			err = cmdLaunch(ctx, paths, accounts, os.Args[2:], false)
		case "boot-test":
			err = cmdLaunch(ctx, paths, accounts, os.Args[2:], true)
		case "client":
			err = cmdClient(ctx, paths, os.Args[2:])
		case "mods":
			err = cmdMods(ctx, paths, os.Args[2:])
		case "help", "-h", "--help":
			usage()
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
			usage()
			os.Exit(2)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `%s %s

Usage:
  %[1]s             start the launcher (TUI)
  %[1]s install   [-mc VERSION] [-loader fabric|vanilla] [-loader-version VERSION]
  %[1]s instances list instances
  %[1]s accounts  list accounts
  %[1]s login     sign in with Microsoft (device code)
  %[1]s launch    -instance ID [-offline NAME] [-mem MB] [-server HOST[:PORT]]
  %[1]s boot-test [-instance ID | -mc VERSION -loader LOADER] [-addons a,b] [-no-client] [-timeout DURATION]
  %[1]s client resolve [-mc VERSION] [-o FILE]   build a client manifest from Modrinth
  %[1]s client build [-src DIR]                  build the client mod and add it to the client layer
  %[1]s mods list|search|install|remove|updates -instance ID [args]   manage an instance's mods
  %[1]s version
  %[1]s update [-check]                          update the launcher and the client
  %[1]s release keygen|manifest|index            build a release (used by the release workflow)
  %[1]s cosmetics give|take|add-cape|add-badge|list|sign|keygen   manage capes and badges (in the repository)
`, app.Name, app.Version)
}

func runTUI() error {
	paths, err := app.DefaultPaths()
	if err != nil {
		return err
	}
	client.Init(paths)
	update.CleanupOld()
	return tui.Run(paths)
}

// setup runs the one-time migrations (as the TUI does) and opens the accounts.
func setup(paths app.Paths) (*auth.Accounts, error) {
	client.Init(paths)
	update.CleanupOld()
	s, err := paths.LoadSettings()
	if err != nil {
		return nil, err
	}
	changed, err := instance.Migrate(paths, &s)
	if err != nil {
		return nil, err
	}
	accounts, accChanged, err := auth.Open(paths, &s)
	if err != nil {
		return nil, err
	}
	if changed || accChanged {
		if err := paths.SaveSettings(s); err != nil {
			return nil, err
		}
	}
	return accounts, nil
}

func cmdAccounts(accounts *auth.Accounts) error {
	list := accounts.List()
	if len(list) == 0 {
		fmt.Println("No accounts yet. Add one with `login`, or in the TUI.")
		return nil
	}
	active, _ := accounts.Active()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\tNAME\tTYPE\tUUID")
	for _, a := range list {
		mark := ""
		if a.ID == active.ID {
			mark = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", mark, a.Name, a.Type, a.ID)
	}
	w.Flush()
	fmt.Printf("\nSign-ins are kept in your %s.\n", accounts.SecretsBackend())
	return nil
}

func cmdLogin(ctx context.Context, accounts *auth.Accounts) error {
	ms := accounts.Microsoft()
	dc, err := ms.StartDeviceLogin(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("To sign in, open %s and enter the code:\n\n    %s\n\nWaiting…\n", dc.VerificationURI, dc.UserCode)
	tok, err := ms.WaitForLogin(ctx, dc)
	if err != nil {
		return err
	}
	acc, _, err := accounts.CompleteLogin(ctx, tok)
	if err != nil {
		return err
	}
	fmt.Printf("Signed in as %s. It's now the active account.\n", acc.Name)
	return nil
}

type specFlags struct{ game.Spec }

func (f *specFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.Minecraft, "mc", instance.DefaultMinecraft, "Minecraft version")
	fs.StringVar(&f.Loader, "loader", game.LoaderFabric, "loader: fabric or vanilla")
	fs.StringVar(&f.LoaderVersion, "loader-version", "", "loader version (default: latest stable)")
}

func cmdInstall(ctx context.Context, paths app.Paths, args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	var sf specFlags
	sf.register(fs)
	fs.Parse(args)
	inst, err := install(ctx, paths, sf.Spec)
	if err != nil {
		return err
	}
	fmt.Printf("Installed %s (Java: %s)\n", inst.Version.ID, inst.Java)
	return nil
}

func install(ctx context.Context, paths app.Paths, spec game.Spec) (*game.Installation, error) {
	pp := &progressPrinter{}
	in := &game.Installer{
		Paths:      paths,
		DL:         download.New(app.UserAgent()),
		OnStep:     pp.step,
		OnProgress: pp.progress,
	}
	inst, err := in.Install(ctx, spec)
	pp.finish()
	return inst, err
}

func cmdInstances(paths app.Paths) error {
	list, err := instance.Store{Root: paths.Instances()}.List()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("No instances yet. Create one in the TUI.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tVERSION\tLAST PLAYED")
	for _, i := range list {
		played := "never"
		if !i.LastPlayed.IsZero() {
			played = i.LastPlayed.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%s\t%s\t%s %s\t%s\n", i.ID, i.Name, i.Loader, i.Minecraft, played)
	}
	return w.Flush()
}

func cmdLaunch(ctx context.Context, paths app.Paths, accounts *auth.Accounts, args []string, bootTest bool) error {
	name := "launch"
	if bootTest {
		name = "boot-test"
	}
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	var sf specFlags
	if bootTest {
		sf.register(fs)
	}
	instID := fs.String("instance", "", "instance id (see `instances`)")
	offline := fs.String("offline", "", "play offline as NAME instead of the active account")
	mem := fs.Int("mem", 0, "max heap in MB (default: the instance's setting)")
	server := fs.String("server", "", "server to join on start")
	timeout := fs.Duration("timeout", 3*time.Minute, "boot-test: give up after this long")
	noClient := fs.Bool("no-client", false, "boot-test: plain Fabric, without the client layer")
	addonsFlag := fs.String("addons", "", "boot-test: comma-separated client add-ons to enable")
	fs.Parse(args)

	if bootTest && *offline == "" {
		*offline = "BootTest"
	}
	var sess *auth.Session
	if *offline != "" {
		if err := auth.ValidateOfflineName(*offline); err != nil {
			return err
		}
		sess = &auth.Session{Name: *offline, UUID: auth.OfflineUUID(*offline), AccessToken: "0", UserType: "legacy", XUID: "0"}
	} else {
		acc, ok := accounts.Active()
		if !ok {
			return errors.New("no account: run `login`, or pass -offline NAME")
		}
		var err error
		if sess, err = accounts.Session(ctx, acc); err != nil {
			return err
		}
	}

	// A boot test without -instance runs the requested spec in a scratch game
	// dir, so it never touches a real instance's mods or saves.
	spec, gameDir, memory := sf.Spec, "", instance.DefaultMemoryMB
	var addons []string
	if *addonsFlag != "" {
		addons = strings.Split(*addonsFlag, ",")
	}
	if *instID != "" {
		inst, err := instance.Store{Root: paths.Instances()}.Get(*instID)
		if err != nil {
			return fmt.Errorf("instance %q: %w", *instID, err)
		}
		spec, gameDir, memory, addons = inst.Spec(), inst.GameDir(), inst.MemoryMB, inst.Addons
	} else if bootTest {
		dir := spec.Loader + "-" + spec.Minecraft
		if spec.Loader == game.LoaderFabric && !*noClient {
			dir = "client-" + spec.Minecraft
		}
		gameDir = filepath.Join(paths.Root, "boot-test", dir)
	} else {
		return errors.New("-instance ID is required (see `instances`)")
	}
	if *mem > 0 {
		memory = *mem
	}

	// Every Fabric instance runs with the client layer.
	var layer *client.Layer
	if spec.Loader == game.LoaderFabric && !*noClient {
		m, err := client.Load(spec.Minecraft)
		if err != nil {
			return err
		}
		if spec.LoaderVersion == "" {
			spec.LoaderVersion = m.FabricLoader
		}
		pp := &progressPrinter{label: "Client " + m.Version}
		var disabled []client.Disabled
		layer, disabled, err = client.Prepare(ctx, download.New(app.UserAgent()), paths, spec.Minecraft, addons, gameDir, pp.progress)
		pp.finish()
		if err != nil {
			return err
		}
		for _, d := range disabled {
			fmt.Printf("Disabled your copy of %s (now %s): the client includes its own\n", d.ClientMod, d.File)
		}
	}

	inst, err := install(ctx, paths, spec)
	if err != nil {
		return err
	}
	opts := game.LaunchOptions{
		Paths:    paths,
		GameDir:  gameDir,
		Player:   game.PlayerFromSession(sess),
		MemoryMB: memory,
		Server:   *server,
	}
	if layer != nil {
		opts.AddMods = layer.Files
	}
	cmd, err := game.Command(inst, opts)
	if err != nil {
		return err
	}

	crashesBefore := game.CrashReports(gameDir)
	start := time.Now()
	g, err := game.Start(cmd)
	if err != nil {
		return err
	}
	fmt.Printf("Started %s as %s (pid %d), game dir %s\n", inst.Version.ID, sess.Name, cmd.Process.Pid, gameDir)

	if !bootTest {
		interrupted := ctx.Done()
		for {
			select {
			case line, ok := <-g.Lines:
				if !ok {
					if err := <-g.Done; err != nil {
						return fmt.Errorf("game exited: %w", err)
					}
					return nil
				}
				fmt.Println(line)
			case <-interrupted:
				g.Stop(10 * time.Second)
				interrupted = nil
			}
		}
	}

	deadline := time.After(*timeout)
	var tail []string
	clientReady := false
	stopAndWait := func() {
		g.Stop(10 * time.Second)
		for range g.Lines {
		}
		<-g.Done
	}
	fail := func(reason string) error {
		stopAndWait()
		fmt.Fprintln(os.Stderr, "---- last log lines ----")
		for _, l := range tail {
			fmt.Fprintln(os.Stderr, l)
		}
		return fmt.Errorf("boot test failed after %s: %s", time.Since(start).Round(time.Second), reason)
	}
	for {
		select {
		case line, ok := <-g.Lines:
			if !ok {
				err := <-g.Done
				return fail(fmt.Sprintf("game exited before finishing startup (%v)", err))
			}
			tail = append(tail, line)
			if len(tail) > 60 {
				tail = tail[1:]
			}
			if marker := game.FailureMarker(line); marker != "" {
				return fail(marker)
			}
			if layer != nil && layer.Has(client.ModID) && client.IsReadyLine(line) {
				clientReady = true
			}
			if strings.Contains(line, game.StartedMarker) {
				if layer != nil && layer.Has(client.ModID) && !clientReady {
					return fail(fmt.Sprintf("the %s mod didn't report that it was ready", client.ModID))
				}
				stopAndWait()
				if crashes := game.NewCrashReports(gameDir, crashesBefore); len(crashes) > 0 {
					return fmt.Errorf("boot test: game started but wrote crash reports: %v", crashes)
				}
				fmt.Printf("Boot test passed: %s started in %s\n", inst.Version.ID, time.Since(start).Round(time.Second))
				return nil
			}
		case <-deadline:
			return fail(fmt.Sprintf("timed out waiting for %q", game.StartedMarker))
		case <-ctx.Done():
			return fail("interrupted")
		}
	}
}

// Mods in every client release. Required dependencies (Fabric API) are
// added automatically.
var (
	clientCore     = []string{"fabric-api", "sodium", "lithium", "ferrite-core", "immediatelyfast", "entityculling", "dynamic-fps", "modmenu"}
	clientOptional = []string{"iris", "sodium-extra", "reeses-sodium-options", "moreculling", "badoptimizations", "flashback"}

	// Legacy Fabric versions (before 1.14) have none of the above; this is
	// the best of what exists for them on Modrinth.
	legacyClientCore     = []string{"legacy-fabric-api", "phosphor-legacy", "menufpsunlocker", "nnbsp-fix"}
	legacyClientOptional = []string{"legacy-mouse-tweaks"}
)

func cmdClient(ctx context.Context, paths app.Paths, args []string) error {
	if len(args) > 0 && args[0] == "build" {
		return cmdClientBuild(ctx, paths, args[1:])
	}
	if len(args) == 0 || args[0] != "resolve" {
		return errors.New("usage: client resolve [-mc VERSION] [-o FILE] | client build [-src DIR]")
	}
	fs := flag.NewFlagSet("client resolve", flag.ExitOnError)
	mc := fs.String("mc", instance.DefaultMinecraft, "Minecraft version")
	out := fs.String("o", "", "write the manifest here (default: stdout)")
	fs.Parse(args[1:])

	dl := download.New(app.UserAgent())
	in := &game.Installer{Paths: paths, DL: dl}
	loader, err := in.LatestLoader(ctx, game.LoaderFabric, *mc)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", app.Name+"-resolve-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	core, optional := clientCore, clientOptional
	if game.IsLegacyFabric(*mc) {
		core, optional = legacyClientCore, legacyClientOptional
	}
	m, err := client.Resolve(ctx, &modrinth.Client{DL: dl}, dl, tmp, client.ResolveRequest{
		Client: app.Name, Version: app.Version, Minecraft: *mc, FabricLoader: loader,
		Core: core, Optional: optional,
	})
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	for _, mod := range m.Mods {
		kind := "core"
		if mod.Optional {
			kind = "add-on"
		}
		fmt.Fprintf(os.Stderr, "%-7s %-24s %s\n", kind, mod.ID, mod.Version)
	}
	if *out == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(*out, data, 0o644)
}

// cmdClientBuild builds the client mod for one Minecraft version with its
// Gradle wrapper and copies the jar into that version's client layer dev
// folder. It builds with the newest Java runtime the launcher installs (the
// default version's), which can target every older version's Java too.
func cmdClientBuild(ctx context.Context, paths app.Paths, args []string) error {
	fs := flag.NewFlagSet("client build", flag.ExitOnError)
	src := fs.String("src", "client-mod", "client mod source directory")
	mc := fs.String("mc", instance.DefaultMinecraft, "Minecraft version the build is for")
	fs.Parse(args)

	if _, err := client.Load(*mc); err != nil {
		return err
	}
	newest, err := client.Load(instance.DefaultMinecraft)
	if err != nil {
		return err
	}
	v, err := install(ctx, paths, game.Spec{Minecraft: newest.Minecraft, Loader: game.LoaderFabric, LoaderVersion: newest.FabricLoader})
	if err != nil {
		return err
	}
	gradlew := "./gradlew"
	if runtime.GOOS == "windows" {
		gradlew = "gradlew.bat"
	}
	// One Gradle project per Minecraft version (see client-mod/settings.gradle).
	cmd := exec.CommandContext(ctx, gradlew, ":"+*mc+":build", "--no-daemon")
	cmd.Dir = *src
	// The Java runtime's bin dir holds java; JAVA_HOME is its parent.
	cmd.Env = append(os.Environ(), "JAVA_HOME="+filepath.Dir(filepath.Dir(v.Java)))
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gradle build: %w", err)
	}

	libs := filepath.Join(*src, "versions", *mc, "build", "libs")
	jars, _ := filepath.Glob(filepath.Join(libs, client.ModID+"-*.jar"))
	jars = slices.DeleteFunc(jars, func(j string) bool { return strings.HasSuffix(j, "-sources.jar") })
	if len(jars) == 0 {
		return fmt.Errorf("no built jar in %s", libs)
	}
	// Older builds (other versions) may still be there: take the one just built.
	slices.SortFunc(jars, func(a, b string) int {
		ia, _ := os.Stat(a)
		ib, _ := os.Stat(b)
		return ib.ModTime().Compare(ia.ModTime())
	})
	jars = jars[:1]
	dev := client.DevDir(paths, *mc)
	if err := os.MkdirAll(dev, 0o755); err != nil {
		return err
	}
	old, _ := filepath.Glob(filepath.Join(dev, client.ModID+"-*.jar"))
	for _, o := range old {
		os.Remove(o)
	}
	data, err := os.ReadFile(jars[0])
	if err != nil {
		return err
	}
	dst := filepath.Join(dev, filepath.Base(jars[0]))
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("Added %s to the Minecraft %s client layer (%s)\n", filepath.Base(dst), *mc, dev)
	return nil
}

func cmdMods(ctx context.Context, paths app.Paths, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mods list|search|install|remove|updates -instance ID [args]")
	}
	sub := args[0]
	fs := flag.NewFlagSet("mods "+sub, flag.ExitOnError)
	instID := fs.String("instance", "", "instance id (see `instances`)")
	yes := fs.Bool("y", false, "don't ask before changing anything")
	fs.Parse(args[1:])
	if *instID == "" {
		return errors.New("-instance ID is required")
	}
	inst, err := instance.Store{Root: paths.Instances()}.Get(*instID)
	if err != nil {
		return err
	}
	dl := download.New(app.UserAgent())
	mgr, err := mods.Open(ctx, paths, inst, dl)
	if err != nil {
		return err
	}
	mr := &modrinth.Client{DL: dl}
	confirm := func(question string) bool {
		if *yes {
			return true
		}
		fmt.Print(question + " [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		return strings.EqualFold(answer, "y")
	}
	find := func(name string) (*mods.Installed, error) {
		list, err := mgr.Scan()
		if err != nil {
			return nil, err
		}
		for _, i := range list {
			if strings.EqualFold(i.Name(), name) || i.Key == name || i.File == name || (len(i.Units) > 0 && i.Units[0].ID == name) {
				return i, nil
			}
		}
		return nil, fmt.Errorf("no installed mod %q", name)
	}

	switch sub {
	case "list":
		if err := mgr.Identify(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "warning: couldn't identify mods:", err)
		}
		list, err := mgr.Scan()
		if err != nil {
			return err
		}
		for _, mod := range mgr.Layer.Mods {
			fmt.Printf("  [client] %-28s %s\n", mod.Name, mod.Version)
		}
		for _, i := range list {
			state, src := "on ", "local file"
			if !i.Enabled {
				state = "off"
			}
			if i.Source != nil {
				src = i.Source.Source
			}
			fmt.Printf("  [%s]    %-28s %-22s %s\n", state, i.Name(), i.Version(), src)
			for _, p := range i.Problems {
				fmt.Printf("           ! %s\n", p.Text)
			}
		}
	case "search":
		r, err := mr.Search(ctx, modrinth.SearchQuery{Text: strings.Join(fs.Args(), " "), Minecraft: inst.Minecraft, Loader: game.ModrinthLoader(inst.Minecraft), Limit: 15})
		if err != nil {
			return err
		}
		fmt.Printf("%d results for Minecraft %s (Fabric)\n", r.TotalHits, inst.Minecraft)
		for _, h := range r.Hits {
			fmt.Printf("  %-28s %-28s %9d downloads  %s\n", h.Slug, h.Title, h.Downloads, truncate(h.Description, 50))
		}
	case "install":
		if fs.NArg() != 1 {
			return errors.New("usage: mods install -instance ID PROJECT")
		}
		p, err := mr.Project(ctx, fs.Arg(0))
		if err != nil {
			return err
		}
		plan, err := mgr.PlanInstall(ctx, p.ID)
		if err != nil {
			return err
		}
		fmt.Printf("Installing %s:\n", p.Title)
		for _, s := range plan.Steps {
			why := "requested"
			if !s.Requested {
				why = "dependency"
			}
			fmt.Printf("  + %-28s %-20s (%s)\n", s.Name(), s.Version.Number, why)
		}
		for _, w := range plan.Warnings {
			fmt.Printf("  ! %s\n", w.Text)
		}
		for _, n := range plan.Notes {
			fmt.Printf("  · %s\n", n)
		}
		if !confirm("Go ahead?") {
			return nil
		}
		if err := mgr.Commit(plan); err != nil {
			return err
		}
		fmt.Println("Done.")
	case "remove":
		if fs.NArg() != 1 {
			return errors.New("usage: mods remove -instance ID NAME")
		}
		i, err := find(fs.Arg(0))
		if err != nil {
			return err
		}
		r, err := mgr.PlanRemove(i)
		if err != nil {
			return err
		}
		for _, x := range r.Remove {
			fmt.Printf("  - %s\n", x.Name())
		}
		for _, d := range r.Dependents {
			fmt.Printf("  ! %s needs it and won't start without it\n", d)
		}
		if !confirm("Remove?") {
			return nil
		}
		return mgr.CommitRemove(r)
	case "updates":
		list, err := mgr.Scan()
		if err != nil {
			return err
		}
		ups, err := mgr.CheckUpdates(ctx, list)
		if err != nil {
			return err
		}
		if len(ups) == 0 {
			fmt.Println("Everything is up to date.")
		}
		for _, u := range ups {
			fmt.Printf("  %-28s %s → %s\n", u.Installed.Name(), u.Installed.Source.Version, u.Version.Number)
		}
	default:
		return fmt.Errorf("unknown mods command %q", sub)
	}
	return nil
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func cmdUpdate(ctx context.Context, paths app.Paths, args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	check := fs.Bool("check", false, "only report whether an update exists")
	force := fs.Bool("force", false, "update a development build too")
	fs.Parse(args)
	if update.Disabled() && !*force {
		return fmt.Errorf("this is a development build (%s); updates are off (use -force to try anyway)", app.Version)
	}
	dl := download.New(app.UserAgent())
	r, err := update.Fetch(ctx, dl)
	if err != nil {
		return err
	}
	if err := update.RefreshClient(ctx, dl, paths.Root, r); err != nil {
		return fmt.Errorf("client update: %w", err)
	}
	fmt.Printf("Client is at %s.\n", r.Version)
	if !r.Newer() {
		fmt.Printf("Launcher %s is up to date.\n", app.Version)
		return nil
	}
	if *check {
		fmt.Printf("Launcher update available: %s → %s (run `%s update`).\n", app.Version, r.Version, app.Name)
		return nil
	}
	if err := update.ApplyLauncher(ctx, dl, r); err != nil {
		return err
	}
	fmt.Printf("Launcher updated %s → %s. It takes effect the next time you start it.\n", app.Version, r.Version)
	return nil
}

func cmdRelease(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: release keygen FILE | manifest -version V -jar JAR -dist DIR | index -version V -dist DIR")
	}
	fs := flag.NewFlagSet("release "+args[0], flag.ExitOnError)
	version := fs.String("version", "", "release version, without the leading v")
	jar := fs.String("jar", "", "the built client mod jars, with {mc} for the Minecraft version")
	dist := fs.String("dist", "dist", "release output folder")
	manifests := fs.String("manifests", "internal/client/manifests", "client manifests to stamp")
	fs.Parse(args[1:])
	switch args[0] {
	case "keygen":
		if fs.NArg() != 1 {
			return errors.New("usage: release keygen FILE")
		}
		if _, err := os.Stat(fs.Arg(0)); err == nil {
			return fmt.Errorf("refusing to overwrite %s", fs.Arg(0))
		}
		seed, pub, err := update.NewKey()
		if err != nil {
			return err
		}
		if err := os.WriteFile(fs.Arg(0), []byte(seed+"\n"), 0o600); err != nil {
			return err
		}
		fmt.Printf("Private key written to %s (keep it secret).\nPublic key for internal/update/key.go:\n%s\n", fs.Arg(0), pub)
		return nil
	case "manifest":
		if *version == "" || *jar == "" {
			return errors.New("-version and -jar are required")
		}
		return update.StampManifests(*manifests, *dist, strings.TrimPrefix(*version, "v"), *jar)
	case "index":
		key := os.Getenv("ARMSTRONG_SIGNING_KEY")
		if key == "" {
			return errors.New("ARMSTRONG_SIGNING_KEY isn't set")
		}
		r, err := update.WriteIndex(*dist, strings.TrimPrefix(*version, "v"), key)
		if err != nil {
			return err
		}
		fmt.Printf("release.json: %d launchers, %d client manifests, %d mod jars\n", len(r.Launcher), len(r.Client), len(r.Mods))
		return nil
	}
	return fmt.Errorf("unknown release command %q", args[0])
}

// progressPrinter shows install steps and a single updating progress line.
type progressPrinter struct {
	label   string
	last    time.Time
	printed bool
}

func (p *progressPrinter) step(label string) {
	p.finish()
	p.label = label
}

func (p *progressPrinter) progress(pr download.Progress) {
	if pr.Done < pr.Total && time.Since(p.last) < 100*time.Millisecond {
		return
	}
	p.last = time.Now()
	p.printed = true
	fmt.Printf("\r%-20s %5d/%-5d  %6.1f MB downloaded", p.label, pr.Done, pr.Total, float64(pr.Bytes)/1e6)
}

func (p *progressPrinter) finish() {
	if p.printed {
		fmt.Println()
		p.printed = false
	}
}
