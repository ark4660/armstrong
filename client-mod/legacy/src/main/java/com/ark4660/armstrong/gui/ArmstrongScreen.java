package com.ark4660.armstrong.gui;

import com.ark4660.armstrong.compat.Gfx;
import com.ark4660.armstrong.compat.Mc;

import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.ModuleManager;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import org.lwjgl.input.Keyboard;
import org.lwjgl.input.Mouse;

import net.fabricmc.loader.api.FabricLoader;
import net.minecraft.client.gui.screen.Screen;
import net.minecraft.client.option.KeyBinding;
import net.minecraft.util.SharedConstants;

/**
 * The Armstrong menu (Right Shift), drawn as a terminal window. Keyboard
 * first: ↑↓ select, space toggles, enter edits, tab switches category,
 * / searches, e opens the HUD editor; in settings ←→ adjust and r resets.
 * The mouse works too.
 */
public final class ArmstrongScreen extends Screen {
	private static final int BAR = 13;       // title and footer bars
	private static final int SIDEBAR = 96;
	private static final int ROW = 12;
	private static final int BAR_CELLS = 10; // slider width in characters

	private final ModuleManager modules;
	private final KeyBinding menuKey;
	private final String version = FabricLoader.getInstance().getModContainer("armstrong")
			.map(c -> c.getMetadata().getVersion().getFriendlyString()).orElse("");

	private Category category; // null: all
	private String query = "";
	private boolean searching;
	private int selected;
	private int scroll;

	private Module editing;
	private int settingRow;
	private int settingScroll; // first settings row shown
	private Setting.Text editingText; // a text setting being typed into
	private String textBuffer = "";

	private int x0, y0, w, h; // window
	private final List<Hit> hits = new ArrayList<>();
	private double clickX; // where the click being handled landed

	private record Hit(int x, int y, int w, int h, Runnable action) {
		boolean contains(double px, double py) {
			return px >= x && px < x + w && py >= y && py < y + h;
		}
	}

	public ArmstrongScreen(ModuleManager modules, KeyBinding menuKey) {
		this.modules = modules;
		this.menuKey = menuKey;
	}

	@Override
	public void init() {
		Keyboard.enableRepeatEvents(true);
		w = Math.min(width - 16, 440);
		h = Math.min(height - 16, 270);
		x0 = (width - w) / 2;
		y0 = (height - h) / 2;
	}

	// ---- data ----

	private List<Module> visible() {
		String q = query.trim().toLowerCase(Locale.ROOT);
		List<Module> out = new ArrayList<>();
		for (Module m : modules.all()) {
			if (category != null && m.category != category) {
				continue;
			}
			if (!q.isEmpty() && !m.id.contains(q) && !m.name.toLowerCase(Locale.ROOT).contains(q)
					&& !m.description.toLowerCase(Locale.ROOT).contains(q)) {
				continue;
			}
			out.add(m);
		}
		return out;
	}

	private Module current() {
		List<Module> list = visible();
		if (list.isEmpty()) {
			return null;
		}
		selected = Math.clamp(selected, 0, list.size() - 1);
		return list.get(selected);
	}

	/** Rows of the settings view: "enabled", each setting, then the key (read-only). */
	private int settingRows(Module m) {
		return 1 + m.settings().size() + (m.key() != null ? 1 : 0);
	}

	private static String keyName(KeyBinding k) {
		return Mc.keyName(k);
	}

	/** [on ], [off], or [blk] when the server blocks a module that's on. */
	private static String state(Module m) {
		if (!m.isEnabled()) {
			return "[off]";
		}
		return m.blockedBy() != null ? "[blk]" : "[on ]";
	}

	private static String choiceText(String v) {
		return v.toLowerCase(Locale.ROOT).replace(' ', '-');
	}

	private static String num(Setting.Number n, double v) {
		return n.step >= 1 ? String.valueOf(Math.round(v)) : String.format(Locale.ROOT, "%.1f", v);
	}

	// ---- actions ----

	private void cycleCategory(int dir) {
		Category[] all = Category.values();
		int i = category == null ? 0 : category.ordinal() + 1; // 0 = all
		do {
			i = Math.floorMod(i + dir, all.length + 1);
			category = i == 0 ? null : all[i - 1];
		} while (category != null && count(category) == 0);
		selected = 0;
		scroll = 0;
	}

	/** Modules in a category (null: all). 1.8.9 has no replay modules, so that category is hidden. */
	private int count(Category c) {
		int n = 0;
		for (Module m : modules.all()) {
			if (c == null || m.category == c) {
				n++;
			}
		}
		return n;
	}

	private void move(int dir) {
		if (editing != null) {
			settingRow = Math.floorMod(settingRow + dir, settingRows(editing));
		} else {
			int n = visible().size();
			if (n > 0) {
				selected = Math.floorMod(selected + dir, n);
			}
		}
	}

	private void open(Module m) {
		editing = m;
		settingRow = 0;
		settingScroll = 0;
	}

	/** Changes the selected setting: dir -1/+1 for ←→, 0 for space/enter. */
	private void adjust(int dir) {
		Module m = editing;
		if (settingRow == 0) {
			m.toggle();
			return;
		}
		int i = settingRow - 1;
		if (i >= m.settings().size()) {
			return; // the key row: rebind in the game's controls
		}
		switch (m.settings().get(i)) {
			case Setting.Bool b -> b.toggle();
			case Setting.Choice c -> {
				int steps = dir < 0 ? c.options.length - 1 : 1;
				for (int k = 0; k < steps; k++) {
					c.next();
				}
			}
			case Setting.Number n -> {
				if (dir != 0) {
					n.set(n.get() + dir * n.step);
				}
			}
			case Setting.Text t -> {
				if (dir == 0) {
					editingText = t;
					textBuffer = t.get();
				}
			}
		}
	}

	private void stopTextEdit(boolean save) {
		if (editingText == null) {
			return;
		}
		if (save) {
			editingText.set(textBuffer.strip());
		}
		editingText = null;
	}

	// ---- input ----

	/**
	 * 1.8.9 delivers a key and its character together. While typing, a
	 * printable character is text; everything else goes to the key handler.
	 */
	@Override
	protected void keyPressed(char c, int code) {
		if ((editingText != null || searching) && c >= ' ' && c != 127) {
			onChar(String.valueOf(c), SharedConstants.isValidChar(c));
		} else if (!onKey(code, hasShiftDown() ? 1 : 0)) {
			super.keyPressed(c, code);
		}
	}

	private boolean onKey(int key, int modifiers) {
		if (editingText != null) {
			switch (key) {
				case Keyboard.KEY_RETURN -> stopTextEdit(true);
				case Keyboard.KEY_ESCAPE -> stopTextEdit(false);
				case Keyboard.KEY_BACK -> {
					if (!textBuffer.isEmpty()) {
						textBuffer = textBuffer.substring(0, textBuffer.offsetByCodePoints(textBuffer.length(), -1));
					}
				}
				default -> {
				}
			}
			return true;
		}
		if (searching) {
			switch (key) {
				case Keyboard.KEY_ESCAPE -> {
					setSearching(false);
					query = "";
					selected = 0;
				}
				case Keyboard.KEY_RETURN, Keyboard.KEY_DOWN -> setSearching(false);
				case Keyboard.KEY_BACK -> {
					if (!query.isEmpty()) {
						query = query.substring(0, query.offsetByCodePoints(query.length(), -1));
						selected = 0;
					}
				}
				default -> {
					if (Mc.isBound(menuKey, key)) {
						onClose();
					}
				}
			}
			return true;
		}
		if (Mc.isBound(menuKey, key)) {
			onClose();
			return true;
		}
		if (editing != null) {
			switch (key) {
				case Keyboard.KEY_ESCAPE, Keyboard.KEY_BACK -> editing = null;
				case Keyboard.KEY_UP -> move(-1);
				case Keyboard.KEY_DOWN -> move(1);
				case Keyboard.KEY_LEFT -> adjust(-1);
				case Keyboard.KEY_RIGHT -> adjust(1);
				case Keyboard.KEY_SPACE, Keyboard.KEY_RETURN -> adjust(0);
				case Keyboard.KEY_R -> editing.reset();
				default -> {
					return false;
				}
			}
			return true;
		}
		Module m = current();
		switch (key) {
			case Keyboard.KEY_ESCAPE -> onClose();
			case Keyboard.KEY_UP -> move(-1);
			case Keyboard.KEY_DOWN -> move(1);
			case Keyboard.KEY_TAB -> cycleCategory(modifiers == 0 ? 1 : -1);
			case Keyboard.KEY_SLASH -> setSearching(true);
			case Keyboard.KEY_E -> openHudEditor();
			case Keyboard.KEY_SPACE -> {
				if (m != null) {
					m.toggle();
				}
			}
			case Keyboard.KEY_RETURN -> {
				if (m != null) {
					open(m);
				}
			}
			default -> {
				return false;
			}
		}
		return true;
	}

	private void openHudEditor() {
		modules.save();
		Mc.open(new HudEditorScreen(modules, menuKey, this));
	}

	private boolean onChar(String c, boolean allowedInChat) {
		if (editingText != null) {
			if (allowedInChat && textBuffer.length() < editingText.maxLength) {
				textBuffer += c;
			}
			return true;
		}
		if (!searching) {
			return false;
		}
		if (allowedInChat && query.length() < 32) {
			query += c;
			selected = 0;
			scroll = 0;
		}
		return true;
	}

	@Override
	protected void mouseClicked(int x, int y, int button) {
		if (button == 0) {
			clickX = x;
			for (int i = hits.size() - 1; i >= 0; i--) {
				if (hits.get(i).contains(x, y)) {
					hits.get(i).action().run();
					return;
				}
			}
		}
		super.mouseClicked(x, y, button);
	}

	@Override
	public void handleMouse() {
		super.handleMouse();
		int wheel = Mouse.getEventDWheel();
		if (wheel != 0) {
			move(wheel > 0 ? -1 : 1);
		}
	}

	private void setSearching(boolean on) {
		searching = on;
	}

	private void onClose() {
		modules.save();
		Mc.open(null);
	}

	@Override
	public void removed() {
		setSearching(false);
		stopTextEdit(false);
		Keyboard.enableRepeatEvents(false);
	}

	@Override
	public boolean shouldPauseGame() {
		return false;
	}

	// ---- drawing ----

	@Override
	public void render(int mouseX, int mouseY, float delta) {
		Gfx g = new Gfx();
		drawBackground(g);
		draw(g);
		super.render(mouseX, mouseY, delta);
	}

	private void drawBackground(Gfx g) {
		renderBackground();
		g.fill(x0, y0, x0 + w, y0 + h, Theme.WINDOW);
		Term.outline(g, x0, y0, w, h, Theme.accent());
	}

	private void draw(Gfx g) {
		hits.clear();
		int accent = Theme.accent();

		// Title bar.
		String path = editing == null ? "~/modules" : "~/modules/" + editing.category.title + "/" + editing.id;
		new Term.Line().add("armstrong", accent).add("@client: " + path, Theme.MUTED).draw(g, x0 + 6, y0 + 3);
		String right = editing == null ? "v" + version + " · " + modules.all().size() + " modules" : "config/armstrong.json";
		Term.draw(g, right, x0 + w - 6 - Term.width(right), y0 + 3, Theme.MUTED);
		g.fill(x0 + 1, y0 + BAR, x0 + w - 1, y0 + BAR + 1, Theme.BORDER);

		// Footer.
		int fy = y0 + h - BAR;
		g.fill(x0 + 1, fy, x0 + w - 1, fy + 1, Theme.BORDER);
		Term.Line hints = new Term.Line();
		String[][] keys = searching ? new String[][]{{"type", "filter"}, {"enter", "done"}, {"esc", "clear"}}
				: editing != null ? new String[][]{{"↑↓", "select"}, {"←→", "adjust"}, {"space", "toggle"}, {"r", "reset"}, {"esc", "back"}}
				: new String[][]{{"↑↓", "select"}, {"space", "toggle"}, {"enter", "edit"}, {"e", "hud editor"}, {"tab", "category"}, {"/", "search"}, {"rshift", "close"}};
		int room = w - 12;
		for (String[] k : keys) {
			Term.Line hint = new Term.Line().add(k[0], Theme.AMBER).add(" " + k[1] + "  ", Theme.MUTED);
			if (hints.width() + hint.width() > room) {
				break; // a narrow window drops the last hints
			}
			hints.append(hint);
		}
		hints.draw(g, x0 + 6, fy + 3);

		drawSidebar(g, accent);
		if (editing == null) {
			drawList(g, accent);
		} else {
			drawSettings(g, accent);
		}
	}

	private void drawSidebar(Gfx g, int accent) {
		int x = x0 + 6, y = y0 + BAR + 6;
		g.fill(x0 + SIDEBAR, y0 + BAR + 1, x0 + SIDEBAR + 1, y0 + h - BAR, Theme.BORDER);
		Term.draw(g, "~/modules/", x, y, Theme.MUTED);
		y += ROW + 2;
		int cw = SIDEBAR - 10;
		for (int i = 0; i <= Category.values().length; i++) {
			Category c = i == 0 ? null : Category.values()[i - 1];
			int count = count(c);
			if (count == 0) {
				continue;
			}
			String label = Term.pad((c == null ? "all" : c.title) + "/", 10) + count;
			boolean on = c == category && editing == null;
			if (on) {
				g.fill(x - 2, y - 2, x - 2 + cw, y + ROW - 2, accent);
				Term.draw(g, "> " + label, x, y, Theme.ON_ACCENT);
			} else {
				Term.draw(g, "  " + label, x, y, c == category ? accent : Theme.TEXT);
			}
			final Category target = c;
			hits.add(new Hit(x - 2, y - 2, cw, ROW, () -> {
				category = target;
				editing = null;
				selected = 0;
				scroll = 0;
			}));
			y += ROW + 2;
		}
	}

	private void drawList(Gfx g, int accent) {
		int x = x0 + SIDEBAR + 8, cw = w - SIDEBAR - 16;
		int y = y0 + BAR + 6;

		// Search prompt.
		Term.panel(g, x, y - 2, cw, ROW + 1, Theme.WINDOW, searching ? accent : Theme.BORDER);
		int after = new Term.Line().add("$ ", accent).add("ls | grep ", Theme.MUTED).add(query, Theme.BRIGHT).draw(g, x + 4, y);
		if (searching && (System.currentTimeMillis() / 500) % 2 == 0) {
			g.fill(after, y - 1, after + Term.cell(), y + 8, accent);
		}
		hits.add(new Hit(x, y - 2, cw, ROW + 1, () -> setSearching(true)));
		y += ROW + 6;

		// Table.
		Term.draw(g, "  STATE  " + Term.pad("MODULE", 15) + Term.pad("CATEGORY", 10) + "KEY", x + 2, y, Theme.MUTED);
		y += ROW - 1;
		g.fill(x, y, x + cw, y + 1, Theme.RULE);
		y += 3;

		List<Module> list = visible();
		Module sel = current();
		int descTop = y0 + h - BAR - 4 * ROW - 4; // id, two description lines, a status line
		int fits = Math.max(1, (descTop - y - 4) / ROW);
		if (selected < scroll) {
			scroll = selected;
		} else if (selected >= scroll + fits) {
			scroll = selected - fits + 1;
		}
		scroll = Math.clamp(scroll, 0, Math.max(0, list.size() - fits));
		for (int i = scroll; i < list.size() && i < scroll + fits; i++) {
			Module m = list.get(i);
			boolean isSel = m == sel;
			if (isSel) {
				g.fill(x, y - 2, x + cw, y + ROW - 2, Theme.accentDim());
			}
			new Term.Line().add(isSel ? "> " : "  ", Theme.AMBER)
					.add(state(m), m.blockedBy() != null && m.isEnabled() ? Theme.RED : m.isEnabled() ? accent : Theme.MUTED)
					.add("  " + Term.pad(m.id, 15) + Term.pad(m.category.title, 10) + keyName(m.key()), isSel ? Theme.BRIGHT : Theme.TEXT)
					.draw(g, x + 2, y);
			final int index = i;
			// Clicking the state toggles; clicking the selected row opens its settings.
			hits.add(new Hit(x, y - 2, cw, ROW, () -> {
				if (selected == index) {
					open(m);
				} else {
					selected = index;
				}
			}));
			hits.add(new Hit(x, y - 2, 2 + Term.width("> [off]"), ROW, () -> {
				selected = index;
				m.toggle();
			}));
			y += ROW;
		}
		if (list.isEmpty()) {
			Term.draw(g, "  no modules match \"" + query + "\"", x + 2, y, Theme.MUTED);
		}

		// Description of the selected module.
		g.fill(x, descTop, x + cw, descTop + 1, Theme.RULE);
		if (sel != null) {
			int dy = descTop + 4;
			Term.draw(g, "# " + sel.id, x, dy, Theme.MUTED);
			List<String> desc = Term.wrap(sel.description.toLowerCase(Locale.ROOT), cw, 2);
			for (int i = 0; i < desc.size(); i++) {
				Term.draw(g, desc.get(i), x, dy + (i + 1) * ROW, Theme.TEXT);
			}
			int n = sel.settings().size();
			int sy = dy + (desc.size() + 1) * ROW;
			if (sel.blockedBy() != null) {
				Term.draw(g, Term.fit("blocked on " + sel.blockedBy() + " (server rules) · stays on elsewhere", cw), x, sy, Theme.RED);
			} else {
				Term.draw(g, n + " setting" + (n == 1 ? "" : "s") + " · enter to edit", x, sy, Theme.MUTED);
			}
		}
	}

	private void drawSettings(Gfx g, int accent) {
		Module m = editing;
		int x = x0 + SIDEBAR + 8, cw = w - SIDEBAR - 16;
		int y = y0 + BAR + 6;
		new Term.Line().add("$ ", accent).add("armstrong config " + m.id, Theme.TEXT).draw(g, x, y);
		y += ROW;
		for (String line : Term.wrap("# " + m.description.toLowerCase(Locale.ROOT), cw, 2)) {
			Term.draw(g, line, x, y, Theme.MUTED);
			y += ROW;
		}
		y += 6;

		int nameW = "enabled".length();
		for (Setting<?> s : m.settings()) {
			nameW = Math.max(nameW, s.name.length());
		}
		nameW += 2;
		int rows = settingRows(m);
		settingRow = Math.clamp(settingRow, 0, rows - 1);
		// Rows that fit above the footer note; the view follows the selection.
		// When they don't all fit, the last line says which ones are shown.
		int fits = Math.max(1, (y0 + h - BAR - ROW - 6 - y) / ROW);
		if (rows > fits) {
			fits = Math.max(1, fits - 1);
		}
		if (settingRow < settingScroll) {
			settingScroll = settingRow;
		} else if (settingRow >= settingScroll + fits) {
			settingScroll = settingRow - fits + 1;
		}
		settingScroll = Math.clamp(settingScroll, 0, Math.max(0, rows - fits));
		if (rows > fits) {
			int last = Math.min(rows, settingScroll + fits);
			Term.draw(g, "  " + (settingScroll + 1) + "-" + last + " of " + rows + " · scroll for more", x, y + fits * ROW, Theme.MUTED);
		}
		for (int r = settingScroll; r < rows && r < settingScroll + fits; r++) {
			boolean isSel = r == settingRow;
			if (isSel) {
				g.fill(x, y - 2, x + cw, y + ROW - 2, Theme.accentDim());
			}
			Term.Line line = new Term.Line().add(isSel ? "> " : "  ", Theme.AMBER);
			int valueX;
			if (r == 0) {
				line.add(Term.pad("enabled", nameW) + "= ", isSel ? Theme.BRIGHT : Theme.TEXT);
				valueX = x + line.width();
				if (m.canDisable()) {
					line.add(String.valueOf(m.isEnabled()), m.isEnabled() ? accent : Theme.MUTED);
				} else {
					line.add("always", Theme.MUTED);
				}
			} else if (r - 1 < m.settings().size()) {
				Setting<?> s = m.settings().get(r - 1);
				line.add(Term.pad(s.name, nameW) + "= ", isSel ? Theme.BRIGHT : Theme.TEXT);
				valueX = x + line.width();
				switch (s) {
					case Setting.Bool b -> line.add(String.valueOf(b.get()), b.get() ? accent : Theme.MUTED);
					case Setting.Choice c -> line.add("<" + choiceText(c.get()) + ">", Theme.CYAN);
					case Setting.Text t -> {
						if (t == editingText) {
							line.add(textBuffer, Theme.BRIGHT);
							line.add((System.currentTimeMillis() / 500) % 2 == 0 ? "█" : " ", accent);
							line.add("  enter saves · esc cancels", Theme.MUTED);
						} else {
							line.add(t.display(), t.get().isEmpty() ? Theme.MUTED : Theme.AMBER);
						}
					}
					case Setting.Number n -> {
						line.add(Term.pad(n.display(), 7), Theme.AMBER);
						int barX = x + line.width();
						line.append(Term.bar(n.fraction(), BAR_CELLS, accent));
						line.add("  " + num(n, n.min) + " … " + num(n, n.max), Theme.MUTED);
						// Click on the bar to set the value there.
						int barW = Term.width("[" + "#".repeat(BAR_CELLS) + "]");
						final int row = r;
						hits.add(new Hit(barX, y - 2, barW, ROW, () -> {
							settingRow = row;
							double cellW = Term.cell();
							double mx = clickX - barX - cellW;
							n.setFraction(mx / (cellW * BAR_CELLS));
						}));
					}
				}
			} else {
				line.add(Term.pad("key", nameW) + "= ", isSel ? Theme.BRIGHT : Theme.TEXT);
				valueX = x + line.width();
				line.add(keyName(m.key()), Theme.CYAN).add("  (rebind in controls)", Theme.MUTED);
			}
			line.draw(g, x, y);
			final int row = r;
			final int vx = valueX;
			hits.add(0, new Hit(x, y - 2, cw, ROW, () -> {
				// A click on the value changes it; a click on the name selects the row.
				if (settingRow == row && clickX >= vx) {
					adjust(0);
				}
				settingRow = row;
			}));
			y += ROW;
		}

		int fy = y0 + h - BAR - ROW - 2;
		new Term.Line().add("-- saved on close · ", Theme.MUTED).add("r", accent).add(" resets this module to defaults --", Theme.MUTED)
				.draw(g, x, fy);
	}

}
