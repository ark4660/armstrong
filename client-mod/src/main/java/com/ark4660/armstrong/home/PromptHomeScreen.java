package com.ark4660.armstrong.home;

import com.ark4660.armstrong.compat.Mc;

import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;
import com.mojang.blaze3d.platform.InputConstants;

import net.fabricmc.loader.api.FabricLoader;
import net.minecraft.client.gui.GuiGraphicsExtractor;

/**
 * Home screen B, "minimal prompt": a centred wordmark and one shell prompt.
 * Typing filters the commands (aliases too: play, online, mods...); enter
 * runs the highlighted one.
 */
public final class PromptHomeScreen extends HomeScreen {
	private static final int ROW = 12;
	private static final int SHOWN = 5;
	/** With an empty prompt, these come first. */
	private static final List<String> FEATURED = List.of("singleplayer", "multiplayer", "continue", "rejoin", "realms");
	/** Shown as one-word shortcuts along the bottom. */
	private static final List<String> SHORTCUTS = List.of("options", "armstrong", "language", "accessibility", "quit");

	private String query = "";

	/** The featured commands and the bottom shortcuts, looked up once per init. */
	private List<Action> featured = List.of(), shortcuts = List.of();

	@Override
	protected void init() {
		super.init();
		featured = named(FEATURED);
		shortcuts = named(SHORTCUTS);
		startTyping(); // the prompt always takes typing
	}

	private List<Action> named(List<String> names) {
		List<Action> out = new ArrayList<>();
		for (String name : names) {
			for (Action a : actions) {
				if (a.name().equals(name)) {
					out.add(a);
					break;
				}
			}
		}
		return out;
	}

	@Override
	public void removed() {
		stopTyping();
		super.removed();
	}

	private List<Action> matches() {
		String q = query.trim().toLowerCase(Locale.ROOT);
		List<Action> out = new ArrayList<>();
		if (q.isEmpty()) {
			out.addAll(featured);
		} else {
			for (Action a : actions) {
				if (a.matches(q)) {
					out.add(a);
				}
			}
		}
		return out.size() > SHOWN ? out.subList(0, SHOWN) : out;
	}

	@Override
	protected boolean onKey(int key, int modifiers) {
		if (handleSwitch(key)) {
			return true;
		}
		List<Action> list = matches();
		switch (key) {
			case InputConstants.KEY_UP -> selected = list.isEmpty() ? 0 : Math.floorMod(selected - 1, list.size());
			case InputConstants.KEY_DOWN, InputConstants.KEY_TAB -> selected = list.isEmpty() ? 0 : Math.floorMod(selected + 1, list.size());
			case InputConstants.KEY_RETURN -> {
				if (!list.isEmpty()) {
					run(list.get(Math.clamp(selected, 0, list.size() - 1)));
				}
			}
			case InputConstants.KEY_BACKSPACE -> {
				if (!query.isEmpty()) {
					query = query.substring(0, query.offsetByCodePoints(query.length(), -1));
					selected = 0;
				}
			}
			case InputConstants.KEY_ESCAPE -> {
				query = "";
				selected = 0;
			}
			default -> {
				return false;
			}
		}
		return true;
	}

	@Override
	protected boolean onChar(String c, boolean allowedInChat) {
		if (c.equals("`")) {
			return true; // that's the layout switch, not text
		}
		if (allowedInChat && query.length() < 24) {
			query += c.toLowerCase(Locale.ROOT);
			selected = 0;
		}
		return true;
	}

	@Override
	protected void drawBackground(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		background(g, delta, 0xC8050806);
	}

	@Override
	protected void draw(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		hits.clear();
		int accent = Theme.accent();
		List<Action> list = matches();
		selected = Math.clamp(selected, 0, Math.max(0, list.size() - 1));

		int boxW = Math.min(260, width - 40);
		int wmW = wordmarkWidth(width - 40, 1f);
		int wmH = Math.round(WORDMARK.length * WORDMARK_ROW * (float) wmW / Term.width(WORDMARK[0]));
		int blockH = wmH + 12 + Term.LINE + 14 + (ROW + 4) + 6 + SHOWN * ROW + 12 + Term.LINE;
		int y = Math.max(8, (height - 14 - blockH) / 2);

		wordmark(g, (width - wmW) / 2, y, width - 40, 1f);
		y += wmH + 12;

		String fabric = FabricLoader.getInstance().getModContainer("fabricloader")
				.map(c -> c.getMetadata().getVersion().getFriendlyString()).orElse("?");
		Term.Line status = new Term.Line()
				.add("minecraft " + Mc.versionName() + " · fabric " + fabric
						+ " · " + ArmstrongClient.modules().all().size() + " modules · signed in as ", Theme.MUTED)
				.add(minecraft.getUser().getName(), accent);
		status.draw(g, (width - status.width()) / 2, y);
		y += Term.LINE + 14;

		// The prompt.
		int bx = (width - boxW) / 2;
		Term.panel(g, bx, y - 3, boxW, ROW + 4, Theme.WINDOW, accent);
		int after = new Term.Line().add("$ ", accent).add(query, Theme.BRIGHT).draw(g, bx + 6, y);
		if ((System.currentTimeMillis() / 500) % 2 == 0) {
			g.fill(after, y - 1, after + Term.cell(), y + 8, accent);
		}
		y += ROW + 4 + 6;

		// Matching commands.
		if (list.isEmpty()) {
			Term.draw(g, "  no command \"" + query + "\" · try: play, online, options, quit", bx, y, Theme.MUTED);
		}
		for (int i = 0; i < list.size(); i++) {
			Action a = list.get(i);
			boolean sel = i == selected;
			if (sel) {
				g.fill(bx, y - 2, bx + boxW, y + ROW - 2, Theme.accentDim());
			}
			new Term.Line().add(sel ? "> " : "  ", Theme.AMBER)
					.add(Term.pad(label(a), 14), sel ? Theme.BRIGHT : Theme.TEXT)
					.add(a.hint(), Theme.MUTED)
					.draw(g, bx + 4, y);
			final int index = i;
			hits.add(bx, y - 2, boxW, ROW, () -> {
				selected = index;
				run(list.get(index));
			});
			y += ROW;
		}
		y = Math.max(y, y + (SHOWN - list.size()) * ROW) + 12;

		// One-word shortcuts.
		Term.Line row = new Term.Line();
		List<int[]> spans = new ArrayList<>();
		for (Action a : shortcuts) {
			String word = a.name().equals("armstrong") ? "modules" : a.name();
			int start = row.width();
			row.add(word.substring(0, 1), Theme.AMBER).add(word.substring(1) + "    ", Theme.MUTED);
			spans.add(new int[]{start, Term.width(word), actions.indexOf(a)});
		}
		int rx = (width - row.width()) / 2;
		row.draw(g, rx, y);
		for (int[] s : spans) {
			Action a = actions.get(s[2]);
			hits.add(rx + s[0], y - 2, s[1], ROW, () -> run(a));
		}

		footer(g, false, hints("type", "command", "↑↓", "choose", "enter", "run", "`", "layout"), hints("`", "layout"));
	}

	/** Friendlier names for the prompt view. */
	private static String label(Action a) {
		return switch (a.name()) {
			case "singleplayer" -> "play";
			case "multiplayer" -> "play online";
			case "armstrong" -> "modules";
			default -> a.name();
		};
	}
}
