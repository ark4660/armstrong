package com.ark4660.armstrong.home;

import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.gui.ArmstrongScreen;
import com.ark4660.armstrong.modules.Interface;
import com.ark4660.armstrong.ui.Hits;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;
import com.mojang.blaze3d.platform.InputConstants;
import com.mojang.realmsclient.RealmsMainScreen;

import net.fabricmc.loader.api.FabricLoader;
import net.fabricmc.loader.api.ModContainer;
import net.fabricmc.loader.api.metadata.ModOrigin;
import net.minecraft.client.gui.GuiGraphicsExtractor;
import net.minecraft.client.gui.screens.ConnectScreen;
import net.minecraft.client.gui.screens.Screen;
import net.minecraft.client.gui.screens.multiplayer.JoinMultiplayerScreen;
import net.minecraft.client.gui.screens.options.AccessibilityOptionsScreen;
import net.minecraft.client.gui.screens.options.LanguageSelectScreen;
import net.minecraft.client.gui.screens.options.OptionsScreen;
import net.minecraft.client.gui.screens.worldselection.SelectWorldScreen;
import net.minecraft.client.input.KeyEvent;
import net.minecraft.client.input.MouseButtonEvent;
import net.minecraft.client.multiplayer.ServerData;
import net.minecraft.client.multiplayer.resolver.ServerAddress;
import net.minecraft.network.chat.Component;

/**
 * Base of Armstrong's home screens (they replace the title screen). It holds
 * the menu actions, the boot log, the dimmed panorama and the wordmark; the
 * two layouts only differ in how they arrange them. ` switches layouts.
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

	/** Core client mods, in the order the boot log names them. */
	private static final List<String> STACK = List.of("sodium", "lithium", "ferritecore", "immediatelyfast", "entityculling", "dynamic_fps");

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

	protected HomeScreen() {
		super(Component.literal("Armstrong"));
	}

	@Override
	protected void init() {
		actions = buildActions();
		bootLog = buildBootLog();
		selected = Math.clamp(selected, 0, Math.max(0, actions.size() - 1));
	}

	private List<Action> buildActions() {
		List<Action> out = new ArrayList<>();
		out.add(new Action("singleplayer", "./play --local", () -> minecraft.setScreenAndShow(new SelectWorldScreen(this)), "play", "sp", "worlds"));
		out.add(new Action("multiplayer", "./play --online", () -> minecraft.setScreenAndShow(new JoinMultiplayerScreen(this)), "online", "mp", "servers"));
		Recent r = Recent.get();
		if (r.world != null) {
			Recent.Entry w = r.world;
			out.add(new Action("continue", w.name + " · " + Recent.ago(w.time),
					() -> minecraft.createWorldOpenFlows().openWorld(w.id, () -> minecraft.setScreenAndShow(this)), "resume", "last"));
		}
		if (r.server != null) {
			Recent.Entry s = r.server;
			out.add(new Action("rejoin", s.name + " · " + Recent.ago(s.time),
					() -> ConnectScreen.startConnecting(this, minecraft, ServerAddress.parseString(s.id),
							new ServerData(s.name, s.id, ServerData.Type.OTHER), false, null), "reconnect", "server"));
		}
		out.add(new Action("realms", "", () -> minecraft.setScreenAndShow(new RealmsMainScreen(this))));
		out.add(new Action("armstrong", "~/modules", () -> minecraft.setScreenAndShow(
				new ArmstrongScreen(ArmstrongClient.modules(), ArmstrongClient.menuKey())), "modules", "mods"));
		out.add(new Action("options", "", () -> minecraft.setScreenAndShow(new OptionsScreen(this, minecraft.options)), "settings"));
		out.add(new Action("language", "", () -> minecraft.setScreenAndShow(
				new LanguageSelectScreen(this, minecraft.options, minecraft.getLanguageManager())), "lang"));
		out.add(new Action("accessibility", "", () -> minecraft.setScreenAndShow(new AccessibilityOptionsScreen(this, minecraft.options)), "a11y"));
		out.add(new Action("quit", "", () -> minecraft.stop(), "exit"));
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
		out.add(ok(accent).add("fabric loader " + fabric, Theme.TEXT));
		out.add(ok(accent).add("armstrong " + armstrong, Theme.TEXT).add(" · " + ArmstrongClient.modules().all().size() + " modules", Theme.MUTED));
		out.add(ok(accent).add("performance stack", Theme.TEXT).add(" · " + stackText, Theme.MUTED));
		out.add(ok(accent).add("your mods", Theme.TEXT).add(" · " + yours + " loaded", Theme.MUTED));
		out.add(new Term.Line().add("[", Theme.MUTED).add("info", Theme.AMBER).add("] ", Theme.MUTED)
				.add("signed in as " + minecraft.getUser().getName(), Theme.TEXT));
		return out;
	}

	private static Term.Line ok(int accent) {
		return new Term.Line().add("[", Theme.MUTED).add(" ok ", accent).add("] ", Theme.MUTED);
	}

	private static String version(ModContainer c) {
		return c.getMetadata().getVersion().getFriendlyString();
	}

	// ---- input shared by both layouts ----

	/** ` switches to the other layout (a developer toggle while choosing one). */
	protected boolean handleSwitch(KeyEvent event) {
		if (event.key() != InputConstants.KEY_GRAVE) {
			return false;
		}
		Interface ui = ArmstrongClient.ui();
		ui.setHomeScreen(this instanceof BootHomeScreen ? Interface.HOME_PROMPT : Interface.HOME_BOOT);
		ArmstrongClient.modules().save();
		minecraft.setScreenAndShow(HomeScreens.create(ui.homeScreen()));
		return true;
	}

	protected void run(Action a) {
		a.run().run();
	}

	@Override
	public boolean mouseClicked(MouseButtonEvent event, boolean doubleClick) {
		if (event.button() == InputConstants.MOUSE_BUTTON_LEFT && hits.click(event.x(), event.y())) {
			return true;
		}
		return super.mouseClicked(event, doubleClick);
	}

	@Override
	public boolean shouldCloseOnEsc() {
		return false; // this is the bottom screen: there's nothing to go back to
	}

	// ---- drawing shared by both layouts ----

	/** The vanilla panorama, crisp but dimmed so the terminal text reads. */
	protected void background(GuiGraphicsExtractor g, float delta, int dim) {
		extractPanorama(g, delta);
		g.fill(0, 0, width, height, dim);
	}

	/** Draws the ARMSTRONG wordmark scaled to fit maxWidth; returns its height. */
	protected int wordmark(GuiGraphicsExtractor g, int x, int y, int maxWidth, float maxScale) {
		int w = Term.width(WORDMARK[0]);
		float s = Math.min(maxScale, (float) maxWidth / w);
		g.pose().pushMatrix();
		g.pose().translate(x, y);
		g.pose().scale(s, s);
		for (int i = 0; i < WORDMARK.length; i++) {
			Term.draw(g, WORDMARK[i], 0, i * WORDMARK_ROW, Theme.accent());
		}
		g.pose().popMatrix();
		return Math.round(WORDMARK.length * WORDMARK_ROW * s);
	}

	protected int wordmarkWidth(int maxWidth, float maxScale) {
		int w = Term.width(WORDMARK[0]);
		return Math.round(w * Math.min(maxScale, (float) maxWidth / w));
	}

	/** Bottom bar: the copyright always on the right; the longest hints that fit on the left. */
	protected void footer(GuiGraphicsExtractor g, boolean bar, Term.Line... hintsLongestFirst) {
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
