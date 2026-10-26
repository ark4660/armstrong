package com.ark4660.armstrong.home;

import com.ark4660.armstrong.compat.Mc;

import java.util.List;

import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;
import com.mojang.blaze3d.platform.InputConstants;

import net.minecraft.client.gui.GuiGraphicsExtractor;

/**
 * Home screen A, "boot terminal": wordmark, a boot log of what loaded, the
 * menu as a command list, and a continue panel on the right.
 */
public final class BootHomeScreen extends HomeScreen {
	private static final int ROW = 11;

	@Override
	protected boolean onKey(int key, int modifiers) {
		if (handleSwitch(key)) {
			return true;
		}
		switch (key) {
			case InputConstants.KEY_UP -> selected = Math.floorMod(selected - 1, actions.size());
			case InputConstants.KEY_DOWN -> selected = Math.floorMod(selected + 1, actions.size());
			case InputConstants.KEY_RETURN, InputConstants.KEY_SPACE -> run(actions.get(selected));
			default -> {
				// 1-9 jump straight to an entry.
				int n = key - InputConstants.KEY_1;
				if (n >= 0 && n < 9 && n < actions.size()) {
					selected = n;
					run(actions.get(n));
					return true;
				}
				return false;
			}
		}
		return true;
	}

	@Override
	protected void drawBackground(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		background(g, delta, 0x9E050806);
	}

	@Override
	protected void draw(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		hits.clear();
		int accent = Theme.accent();

		// Top bar.
		g.fill(0, 0, width, 13, Theme.PANEL);
		g.fill(0, 13, width, 14, Theme.BORDER);
		new Term.Line().add("armstrong", accent).add("@client: ~", Theme.MUTED).draw(g, 8, 3);
		Term.Line who = new Term.Line().add("user ", Theme.MUTED).add(minecraft.getUser().getName(), Theme.TEXT)
				.add(" · mc ", Theme.MUTED).add(Mc.versionName(), Theme.TEXT);
		who.draw(g, width - 8 - who.width(), 3);

		// Left column: wordmark, boot log, menu.
		int x = 16;
		int y = 24;
		int leftW = Math.min(width - 32, Math.max(260, width * 3 / 5));
		y += wordmark(g, x, y, leftW, 0.8f) + 10;

		int menuH = actions.size() * ROW;
		int logH = bootLog.size() * Term.LINE + 8;
		if (y + logH + menuH < height - 20) { // the log goes first when space runs out
			for (Term.Line l : bootLog) {
				l.draw(g, x, y);
				y += Term.LINE;
			}
			y += 8;
		}

		int menuTop = y;
		int menuW = 0;
		for (HomeScreen.Action a : actions) {
			menuW = Math.max(menuW, Term.width("> " + Term.pad(a.name(), 15) + a.hint()));
		}
		menuW += 8;
		for (int i = 0; i < actions.size(); i++) {
			HomeScreen.Action a = actions.get(i);
			boolean sel = i == selected;
			if (sel) {
				g.fill(x - 2, y - 2, x - 2 + menuW, y + ROW - 2, Theme.accentDim());
			}
			new Term.Line().add(sel ? "> " : "  ", Theme.AMBER)
					.add(Term.pad(a.name(), 15), sel ? Theme.BRIGHT : Theme.TEXT)
					.add(a.hint(), Theme.MUTED)
					.draw(g, x, y);
			final int index = i;
			hits.add(x - 2, y - 2, menuW, ROW, () -> {
				selected = index;
				run(actions.get(index));
			});
			y += ROW;
		}

		drawContinue(g, accent, x + menuW + 24, menuTop);

		footer(g, true, hints("↑↓", "select", "enter", "run", "1-9", "jump", "`", "layout"), hints("↑↓", "select", "`", "layout"));
	}

	/** The right-hand panel: jump back into the last world or server. */
	private void drawContinue(GuiGraphicsExtractor g, int accent, int minX, int top) {
		List<HomeScreen.Action> resume = actions.stream()
				.filter(a -> a.name().equals("continue") || a.name().equals("rejoin")).toList();
		int pw = 170;
		int px = width - 16 - pw;
		if (px < minX) {
			return; // no room beside the menu
		}
		int lines = 2 + (resume.isEmpty() ? 1 : resume.size()) + 3;
		int ph = lines * Term.LINE + 8;
		int py = top - 2; // level with the menu, clear of the wider boot log above
		Term.panel(g, px, py, pw, ph, Theme.PANEL, Theme.BORDER);
		int y = py + 5;
		Term.draw(g, "# continue", px + 6, y, Theme.MUTED);
		y += Term.LINE + 2;
		if (resume.isEmpty()) {
			Term.draw(g, "nothing played yet", px + 6, y, Theme.MUTED);
			y += Term.LINE;
		}
		for (HomeScreen.Action a : resume) {
			int textW = pw - 12;
			String label = a.hint();
			while (Term.width("> " + label) > textW && label.length() > 4) {
				label = label.substring(0, label.length() - 2) + "…";
			}
			new Term.Line().add("> ", accent).add(label, Theme.TEXT).draw(g, px + 6, y);
			hits.add(px + 2, y - 2, pw - 4, Term.LINE, () -> run(a));
			y += Term.LINE;
		}
		y += 4;
		Term.draw(g, "# tip", px + 6, y, Theme.MUTED);
		y += Term.LINE;
		new Term.Line().add("rshift", Theme.AMBER).add(" in game opens", Theme.TEXT).draw(g, px + 6, y);
		Term.draw(g, "the module menu.", px + 6, y + Term.LINE, Theme.TEXT);
	}
}
