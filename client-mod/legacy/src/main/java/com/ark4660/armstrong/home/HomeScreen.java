package com.ark4660.armstrong.home;

import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;

import org.lwjgl.input.Keyboard;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.compat.Gfx;
import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.gui.ArmstrongScreen;
import com.ark4660.armstrong.mixin.TitleScreenAccessor;
import com.ark4660.armstrong.modules.Interface;
import com.ark4660.armstrong.ui.Hits;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.fabricmc.loader.api.FabricLoader;
import net.fabricmc.loader.api.ModContainer;
import net.fabricmc.loader.api.metadata.ModOrigin;
import net.minecraft.client.gui.screen.ConnectScreen;
import net.minecraft.client.gui.screen.Screen;
import net.minecraft.client.gui.screen.SettingsScreen;
import net.minecraft.client.gui.screen.TitleScreen;
import net.minecraft.client.gui.screen.multiplayer.MultiplayerScreen;
import net.minecraft.client.gui.screen.options.LanguageOptionsScreen;
import net.minecraft.client.gui.screen.world.SelectWorldScreen;
import net.minecraft.client.network.ServerInfo;
import net.minecraft.realms.RealmsBridge;

/**
 * Base of Armstrong's home screens (they replace the title screen). It holds
 * the menu actions, the boot log, the dimmed panorama and the wordmark; the
 * two layouts only differ in how they arrange them. ` switches layouts.
 * (1.8.9 port of the modern builds' home screen.)
 */
public abstract class HomeScreen extends Screen {
	public static final String COPYRIGHT = "Copyright Mojang AB. Do not distribute!";

	static final String[] WORDMARK = {
		" █████╗ ██████╗ ███╗   ███╗███████╗████████╗██████╗  ██████╗ ███╗   ██╗ ██████╗ ",
		"██╔══██╗██╔══██╗████╗ ████║██╔════╝╚══██╔══╝██╔══██╗██╔═══██╗████╗  ██║██╔════╝ ",
		"███████║██████╔╝██╔████╔██║███████╗   ██║   ██████╔╝██║   ██║██╔██╗ ██║██║  ███╗",
		"██╔══██║██╔══██╗██║╚██╔╝██║╚════██║   ██║   ██╔══██╗██║   ██║██║╚██╗██║██║   ██║",
		"██║  ██║██║  ██║██║ ╚═╝ ██║███████║   ██║   ██║  ██║╚██████╔╝██║ ╚████║╚██████╔╝",
		"╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝ ",
	};
	static final int WORDMARK_ROW = 9;

	/** Core client mods, in the order the boot log names them (the 1.8.9 stack). */
	private static final List<String> STACK = List.of("phosphor-legacy", "mfpu", "nnbsp-fix");

	/** Something the menu can do: a name you can type, a hint, and what it runs. */
	public record Action(String name, String hint, Runnable run, String... aliases) {
		boolean matches(String q) {
			if (name.startsWith(q)) {
				return true;
			}
			for (String a : aliases) {
				if (a.startsWith(q)) {
					return true;
				}
			}
			return false;
		}
	}

	protected final Hits hits = new Hits();
	protected List<Action> actions = List.of();
	protected List<Term.Line> bootLog = List.of();
	protected int selected;

	// 1.8.9's panorama is private to the title screen: keep one, unseen, to draw it.
	private TitleScreen panorama;

	@Override
	public void init() {
		Keyboard.enableRepeatEvents(true);
		actions = buildActions();
		bootLog = buildBootLog();
		selected = Math.clamp(selected, 0, Math.max(0, actions.size() - 1));
		if (panorama == null) {
			panorama = new TitleScreen();
		}
		panorama.init(client, width, height);
	}

	@Override
	public void tick() {
		panorama.tick();
	}

	@Override
	public void removed() {
		Keyboard.enableRepeatEvents(false);
		if (panorama != null) {
			panorama.removed();
		}
	}

	private List<Action> buildActions() {
		List<Action> out = new ArrayList<>();
		out.add(new Action("singleplayer", "./play --local", () -> Mc.open(new SelectWorldScreen(this)), "play", "sp", "worlds"));
		out.add(new Action("multiplayer", "./play --online", () -> Mc.open(new MultiplayerScreen(this)), "online", "mp", "servers"));
		Recent r = Recent.get();
		if (r.world != null) {
			Recent.Entry w = r.world;
			out.add(new Action("continue", w.name + " · " + Recent.ago(w.time),
					() -> {
						// A deleted world would crash the integrated server: show the list instead.
						if (client.getCurrentSave().getLevelProperties(w.id) == null) {
							Mc.open(new SelectWorldScreen(this));
						} else {
							client.startIntegratedServer(w.id, w.name, null);
						}
					}, "resume", "last"));
		}
		if (r.server != null) {
			Recent.Entry s = r.server;
			out.add(new Action("rejoin", s.name + " · " + Recent.ago(s.time),
					() -> Mc.open(new ConnectScreen(this, client, new ServerInfo(s.name, s.id, false))), "reconnect", "server"));
		}
		out.add(new Action("realms", "", () -> new RealmsBridge().switchToRealms(this)));
		out.add(new Action("armstrong", "~/modules", () -> Mc.open(
				new ArmstrongScreen(ArmstrongClient.modules(), ArmstrongClient.menuKey())), "modules", "mods"));
		out.add(new Action("options", "", () -> Mc.open(new SettingsScreen(this, client.options)), "settings"));
		out.add(new Action("language", "", () -> Mc.open(
				new LanguageOptionsScreen(this, client.options, client.getLanguageManager())), "lang"));
		out.add(new Action("quit", "", () -> client.scheduleStop(), "exit"));
		return out;
	}

	private List<Term.Line> buildBootLog() {
		FabricLoader loader = FabricLoader.getInstance();
		String fabric = loader.getModContainer("fabricloader").map(HomeScreen::version).orElse("?");
		String armstrong = loader.getModContainer(ArmstrongClient.MOD_ID).map(HomeScreen::version).orElse("?");
		Path modsDir = loader.getGameDir().resolve("mods").toAbsolutePath().normalize();
		List<String> stack = new ArrayList<>();
		for (String id : STACK) {
			if (loader.isModLoaded(id)) {
				stack.add(id);
			}
		}
		int yours = 0;
		for (ModContainer c : loader.getAllMods()) {
			if (c.getOrigin().getKind() == ModOrigin.Kind.PATH) {
				for (Path p : c.getOrigin().getPaths()) {
					if (modsDir.equals(p.toAbsolutePath().normalize().getParent())) {
						yours++;
						break;
					}
				}
			}
		}
		String stackText = String.join(" ", stack.subList(0, Math.min(3, stack.size())));
		if (stack.size() > 3) {
			stackText += " +" + (stack.size() - 3);
		}
		int accent = Theme.accent();
		List<Term.Line> out = new ArrayList<>();
		out.add(ok(accent).add("legacy fabric loader " + fabric, Theme.TEXT));
		out.add(ok(accent).add("armstrong " + armstrong, Theme.TEXT).add(" · " + ArmstrongClient.modules().all().size() + " modules", Theme.MUTED));
		out.add(ok(accent).add("performance stack", Theme.TEXT).add(" · " + stackText, Theme.MUTED));
		out.add(ok(accent).add("your mods", Theme.TEXT).add(" · " + yours + " loaded", Theme.MUTED));
		out.add(new Term.Line().add("[", Theme.MUTED).add("info", Theme.AMBER).add("] ", Theme.MUTED)
				.add("signed in as " + client.getSession().getUsername(), Theme.TEXT));
		return out;
	}

	private static Term.Line ok(int accent) {
		return new Term.Line().add("[", Theme.MUTED).add(" ok ", accent).add("] ", Theme.MUTED);
	}

	private static String version(ModContainer c) {
		return c.getMetadata().getVersion().getFriendlyString();
	}

	// ---- input shared by both layouts ----

	/** A key press (LWJGL 2 code); return false if unused. */
	protected abstract boolean onKey(int key);

	/** A typed character, after onKey passed on it; return false if unused. */
	protected boolean onChar(char c) {
		return false;
	}

	// 1.8.9 delivers a key and its character together. Escape never closes:
	// this is the bottom screen, there's nothing to go back to.
	@Override
	protected void keyPressed(char c, int code) {
		if (!handleSwitch(code) && !onKey(code)) {
			onChar(c);
		}
	}

	/** ` switches to the other layout (a developer toggle while choosing one). */
	protected boolean handleSwitch(int key) {
		if (key != Keyboard.KEY_GRAVE) {
			return false;
		}
		Interface ui = ArmstrongClient.ui();
		ui.setHomeScreen(this instanceof BootHomeScreen ? Interface.HOME_PROMPT : Interface.HOME_BOOT);
		ArmstrongClient.modules().save();
		Mc.open(HomeScreens.create(ui.homeScreen()));
		return true;
	}

	protected void run(Action a) {
		a.run().run();
	}

	@Override
	protected void mouseClicked(int x, int y, int button) {
		if (button == 0) {
			hits.click(x, y);
		}
	}

	@Override
	public boolean shouldPauseGame() {
		return false;
	}

	// ---- drawing shared by both layouts ----

	@Override
	public void render(int mouseX, int mouseY, float delta) {
		Gfx g = new Gfx();
		drawBackground(g, mouseX, mouseY, delta);
		draw(g);
		super.render(mouseX, mouseY, delta);
	}

	protected abstract void drawBackground(Gfx g, int mouseX, int mouseY, float delta);

	protected abstract void draw(Gfx g);

	/** The vanilla panorama, dimmed so the terminal text reads. */
	protected void background(Gfx g, int mouseX, int mouseY, float delta, int dim) {
		((TitleScreenAccessor) panorama).armstrong$renderPanorama(mouseX, mouseY, delta);
		g.fill(0, 0, width, height, dim);
	}

	/** Draws the ARMSTRONG wordmark scaled to fit maxWidth; returns its height. */
	protected int wordmark(Gfx g, int x, int y, int maxWidth, float maxScale) {
		int w = Term.width(WORDMARK[0]);
		float s = Math.min(maxScale, (float) maxWidth / w);
		g.push();
		g.translate(x, y);
		g.scale(s);
		int cell = Term.cell();
		for (int i = 0; i < WORDMARK.length; i++) {
			String row = WORDMARK[i];
			for (int j = 0; j < row.length(); j++) {
				paintCell(g, row.charAt(j), j * cell, i * WORDMARK_ROW, cell, Theme.accent());
			}
		}
		g.pop();
		return Math.round(WORDMARK.length * WORDMARK_ROW * s);
	}

	/**
	 * One wordmark character as rectangles. 1.8.9 draws block and box-drawing
	 * characters from blurry low-resolution glyph pages, so they're painted:
	 * █ as a solid cell, the shadow (═ ║ ╔ ╗ ╚ ╝) as double 1px lines.
	 */
	private static void paintCell(Gfx g, char c, int x, int y, int w, int color) {
		int h = WORDMARK_ROW;
		int a = w / 2 - 1, b = w / 2 + 1; // the two vertical lines
		int t = h / 2 - 1, u = h / 2 + 1; // the two horizontal lines
		switch (c) {
			case '█' -> g.fill(x, y, x + w, y + h, color);
			case '═' -> {
				g.fill(x, y + t, x + w, y + t + 1, color);
				g.fill(x, y + u, x + w, y + u + 1, color);
			}
			case '║' -> {
				g.fill(x + a, y, x + a + 1, y + h, color);
				g.fill(x + b, y, x + b + 1, y + h, color);
			}
			case '╗' -> { // from the left, turning down
				g.fill(x, y + t, x + b + 1, y + t + 1, color);
				g.fill(x + b, y + t, x + b + 1, y + h, color);
				g.fill(x, y + u, x + a + 1, y + u + 1, color);
				g.fill(x + a, y + u, x + a + 1, y + h, color);
			}
			case '╔' -> { // from the right, turning down
				g.fill(x + a, y + t, x + w, y + t + 1, color);
				g.fill(x + a, y + t, x + a + 1, y + h, color);
				g.fill(x + b, y + u, x + w, y + u + 1, color);
				g.fill(x + b, y + u, x + b + 1, y + h, color);
			}
			case '╝' -> { // from above, turning left
				g.fill(x, y + u, x + b + 1, y + u + 1, color);
				g.fill(x + b, y, x + b + 1, y + u + 1, color);
				g.fill(x, y + t, x + a + 1, y + t + 1, color);
				g.fill(x + a, y, x + a + 1, y + t + 1, color);
			}
			case '╚' -> { // from above, turning right
				g.fill(x + a, y + u, x + w, y + u + 1, color);
				g.fill(x + a, y, x + a + 1, y + u + 1, color);
				g.fill(x + b, y + t, x + w, y + t + 1, color);
				g.fill(x + b, y, x + b + 1, y + t + 1, color);
			}
			default -> {
			}
		}
	}

	protected int wordmarkWidth(int maxWidth, float maxScale) {
		int w = Term.width(WORDMARK[0]);
		return Math.round(w * Math.min(maxScale, (float) maxWidth / w));
	}

	/** Bottom bar: the copyright always on the right; the longest hints that fit on the left. */
	protected void footer(Gfx g, boolean bar, Term.Line... hintsLongestFirst) {
		int y = height - 11;
		if (bar) {
			g.fill(0, height - 14, width, height, Theme.PANEL);
			g.fill(0, height - 14, width, height - 13, Theme.BORDER);
		}
		int cw = Term.width(COPYRIGHT);
		Term.draw(g, COPYRIGHT, width - 8 - cw, y, Theme.MUTED);
		for (Term.Line hints : hintsLongestFirst) {
			if (hints.width() + cw + 24 < width) {
				hints.draw(g, 8, y);
				break;
			}
		}
	}

	/** Builds a key-hint line: pairs of key, description. */
	protected static Term.Line hints(String... pairs) {
		Term.Line l = new Term.Line();
		for (int i = 0; i + 1 < pairs.length; i += 2) {
			l.add(pairs[i], Theme.AMBER).add(" " + pairs[i + 1] + "   ", Theme.MUTED);
		}
		return l;
	}
}
