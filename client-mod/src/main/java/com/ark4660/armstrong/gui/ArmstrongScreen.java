package com.ark4660.armstrong.gui;

import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.ModuleManager;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;
import com.mojang.blaze3d.platform.InputConstants;

import net.fabricmc.loader.api.FabricLoader;
import net.minecraft.client.KeyMapping;
import net.minecraft.client.gui.GuiGraphicsExtractor;
import net.minecraft.client.gui.screens.Screen;
import net.minecraft.client.input.CharacterEvent;
import net.minecraft.client.input.KeyEvent;
import net.minecraft.client.input.MouseButtonEvent;
import net.minecraft.network.chat.Component;

/**
 * The Armstrong menu (Right Shift), drawn as a terminal window. Keyboard
 * first: ↑↓ select, space toggles, enter edits, tab switches category,
 * / searches; in settings ←→ adjust and r resets. The mouse works too.
 */
public final class ArmstrongScreen extends Screen {
	private static final int BAR = 13;       // title and footer bars
	private static final int SIDEBAR = 96;
	private static final int ROW = 12;
	private static final int BAR_CELLS = 10; // slider width in characters

	private final ModuleManager modules;
	private final KeyMapping menuKey;
	private final String version = FabricLoader.getInstance().getModContainer("armstrong")
			.map(c -> c.getMetadata().getVersion().getFriendlyString()).orElse("");

	private Category category; // null: all
	private String query = "";
	private boolean searching;
	private boolean swallowSlash; // the "/" that opened search also arrives as a typed char
	private int selected;
	private int scroll;

	private Module editing;
	private int settingRow;
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

	public ArmstrongScreen(ModuleManager modules, KeyMapping menuKey) {
		super(Component.literal("Armstrong"));
		this.modules = modules;
		this.menuKey = menuKey;
	}

	@Override
	protected void init() {
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

	private static String keyName(KeyMapping k) {
		if (k == null) {
			return "-";
		}
		if (k.isUnbound()) {
			return "none";
		}
		return k.getTranslatedKeyMessage().getString().toLowerCase(Locale.ROOT).replace(' ', '_');
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
		i = Math.floorMod(i + dir, all.length + 1);
		category = i == 0 ? null : all[i - 1];
		selected = 0;
		scroll = 0;
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
					minecraft.textInputManager().startTextInput(this);
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
		minecraft.textInputManager().stopTextInput(this);
	}

	// ---- input ----

	@Override
	public boolean keyPressed(KeyEvent event) {
		int key = event.key();
		if (editingText != null) {
			switch (key) {
				case InputConstants.KEY_RETURN -> stopTextEdit(true);
				case InputConstants.KEY_ESCAPE -> stopTextEdit(false);
				case InputConstants.KEY_BACKSPACE -> {
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
				case InputConstants.KEY_ESCAPE -> {
					setSearching(false);
					query = "";
					selected = 0;
				}
				case InputConstants.KEY_RETURN, InputConstants.KEY_DOWN -> setSearching(false);
				case InputConstants.KEY_BACKSPACE -> {
					if (!query.isEmpty()) {
						query = query.substring(0, query.offsetByCodePoints(query.length(), -1));
						selected = 0;
					}
				}
				default -> {
					if (menuKey.matches(event)) {
						onClose();
					}
				}
			}
			return true;
		}
		if (menuKey.matches(event)) {
			onClose();
			return true;
		}
		if (editing != null) {
			switch (key) {
				case InputConstants.KEY_ESCAPE, InputConstants.KEY_BACKSPACE -> editing = null;
				case InputConstants.KEY_UP -> move(-1);
				case InputConstants.KEY_DOWN -> move(1);
				case InputConstants.KEY_LEFT -> adjust(-1);
				case InputConstants.KEY_RIGHT -> adjust(1);
				case InputConstants.KEY_SPACE, InputConstants.KEY_RETURN -> adjust(0);
				case InputConstants.KEY_R -> editing.reset();
				default -> {
					return super.keyPressed(event);
				}
			}
			return true;
		}
		Module m = current();
		switch (key) {
			case InputConstants.KEY_ESCAPE -> onClose();
			case InputConstants.KEY_UP -> move(-1);
			case InputConstants.KEY_DOWN -> move(1);
			case InputConstants.KEY_TAB -> cycleCategory(event.modifiers() == 0 ? 1 : -1);
			case InputConstants.KEY_SLASH -> {
				setSearching(true);
				swallowSlash = true;
			}
			case InputConstants.KEY_SPACE -> {
				if (m != null) {
					m.toggle();
				}
			}
			case InputConstants.KEY_RETURN -> {
				if (m != null) {
					open(m);
				}
			}
			default -> {
				return super.keyPressed(event);
			}
		}
		return true;
	}

	@Override
	public boolean charTyped(CharacterEvent event) {
		if (editingText != null) {
			if (event.isAllowedChatCharacter() && textBuffer.length() < editingText.maxLength) {
				textBuffer += event.codepointAsString();
			}
			return true;
		}
		if (!searching) {
			return false;
		}
		String c = event.codepointAsString();
		if (swallowSlash && c.equals("/")) {
			swallowSlash = false;
			return true;
		}
		swallowSlash = false;
		if (event.isAllowedChatCharacter() && query.length() < 32) {
			query += c;
			selected = 0;
			scroll = 0;
		}
		return true;
	}

	@Override
	public boolean mouseClicked(MouseButtonEvent event, boolean doubleClick) {
		if (event.button() == InputConstants.MOUSE_BUTTON_LEFT) {
			clickX = event.x();
			for (int i = hits.size() - 1; i >= 0; i--) {
				if (hits.get(i).contains(event.x(), event.y())) {
					hits.get(i).action().run();
					return true;
				}
			}
		}
		return super.mouseClicked(event, doubleClick);
	}

	@Override
	public boolean mouseScrolled(double x, double y, double horizontal, double vertical) {
		if (vertical != 0) {
			move(vertical > 0 ? -1 : 1);
			return true;
		}
		return false;
	}

	/**
	 * Typed characters only arrive while text input is on (26.x uses SDL,
	 * which delivers text only after it's started), so the search prompt
	 * switches it on and off like the game's own text boxes do.
	 */
	private void setSearching(boolean on) {
		if (on == searching) {
			return;
		}
		searching = on;
		if (on) {
			minecraft.textInputManager().startTextInput(this);
		} else {
			minecraft.textInputManager().stopTextInput(this);
		}
	}

	@Override
	public void onClose() {
		modules.save();
		super.onClose();
	}

	@Override
	public void removed() {
		setSearching(false);
		stopTextEdit(false);
		super.removed();
	}

	@Override
	public boolean isPauseScreen() {
		return false;
	}

	// ---- drawing ----

	@Override
	public void extractBackground(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		super.extractBackground(g, mouseX, mouseY, delta);
		g.fill(x0, y0, x0 + w, y0 + h, Theme.WINDOW);
		g.outline(x0, y0, w, h, Theme.accent());
	}

	@Override
	public void extractRenderState(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		super.extractRenderState(g, mouseX, mouseY, delta);
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
				: new String[][]{{"↑↓", "select"}, {"space", "toggle"}, {"enter", "edit"}, {"tab", "category"}, {"/", "search"}, {"rshift", "close"}};
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

	private void drawSidebar(GuiGraphicsExtractor g, int accent) {
		int x = x0 + 6, y = y0 + BAR + 6;
		g.fill(x0 + SIDEBAR, y0 + BAR + 1, x0 + SIDEBAR + 1, y0 + h - BAR, Theme.BORDER);
		Term.draw(g, "~/modules/", x, y, Theme.MUTED);
		y += ROW + 2;
		int cw = SIDEBAR - 10;
		for (int i = 0; i <= Category.values().length; i++) {
			Category c = i == 0 ? null : Category.values()[i - 1];
			int count = 0;
			for (Module m : modules.all()) {
				if (c == null || m.category == c) {
					count++;
				}
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

	private void drawList(GuiGraphicsExtractor g, int accent) {
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
		int descTop = y0 + h - BAR - 4 * ROW;
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
					.add(m.isEnabled() ? "[on ]" : "[off]", m.isEnabled() ? accent : Theme.MUTED)
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
			int dy = descTop + 5;
			Term.draw(g, "# " + sel.id, x, dy, Theme.MUTED);
			Term.draw(g, sel.description.toLowerCase(Locale.ROOT), x, dy + ROW, Theme.TEXT);
			int n = sel.settings().size();
			Term.draw(g, n + " setting" + (n == 1 ? "" : "s") + " · enter to edit", x, dy + 2 * ROW, Theme.MUTED);
		}
	}

	private void drawSettings(GuiGraphicsExtractor g, int accent) {
		Module m = editing;
		int x = x0 + SIDEBAR + 8, cw = w - SIDEBAR - 16;
		int y = y0 + BAR + 6;
		new Term.Line().add("$ ", accent).add("armstrong config " + m.id, Theme.TEXT).draw(g, x, y);
		y += ROW;
		Term.draw(g, "# " + m.description.toLowerCase(Locale.ROOT), x, y, Theme.MUTED);
		y += ROW + 6;

		int nameW = "enabled".length();
		for (Setting<?> s : m.settings()) {
			nameW = Math.max(nameW, s.name.length());
		}
		nameW += 2;
		int rows = settingRows(m);
		settingRow = Math.clamp(settingRow, 0, rows - 1);
		for (int r = 0; r < rows; r++) {
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
