package com.ark4660.armstrong.waypoints;

import java.util.List;

import com.ark4660.armstrong.compat.CompatScreen;
import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.ui.Hits;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;
import org.lwjgl.input.Keyboard;

import net.minecraft.client.MinecraftClient;
import com.ark4660.armstrong.compat.Gfx;

/**
 * This world's waypoints as a terminal list: ↑↓ select, enter renames,
 * space shows/hides, c changes colour, a adds one here, d deletes.
 * Opened with a waypoint to rename (just added) or none (the list key).
 */
public final class WaypointsScreen extends CompatScreen {
	private static final int ROW = 12, BAR = 13;

	private final Waypoints module;
	private final Hits hits = new Hits();
	private List<Waypoint> list = List.of();
	private int selected;
	private int scroll;
	private Waypoint renaming;
	private String buffer = "";
	// The key that started renaming (b in game, a here) may also arrive as a
	// typed char right after; it's dropped only within this short window.
	private long swallowUntil;

	private int x0, y0, w, h;

	public WaypointsScreen(Waypoints module, Waypoint rename) {
		this.module = module;
		if (rename != null) {
			renaming = rename;
			buffer = rename.name;
			swallowUntil = System.currentTimeMillis() + 150;
		}
	}

	@Override
	public void init() {
		String key = WaypointStore.currentKey(MinecraftClient.getInstance());
		list = key == null ? List.of() : module.store().list(key);
		if (renaming != null) {
			selected = Math.max(0, list.indexOf(renaming));
			startTyping();
		}
		w = Math.min(width - 16, 340);
		h = Math.min(height - 16, 220);
		x0 = (width - w) / 2;
		y0 = (height - h) / 2;
	}

	@Override
	public void removed() {
		stopRename(true);
		module.store().save();
		super.removed();
	}

	@Override
	public boolean isPauseScreen() {
		return false;
	}

	private Waypoint current() {
		if (list.isEmpty()) {
			return null;
		}
		selected = Math.clamp(selected, 0, list.size() - 1);
		return list.get(selected);
	}

	private void startRename(Waypoint p) {
		renaming = p;
		buffer = p.name;
		startTyping();
	}

	private void stopRename(boolean keep) {
		if (renaming == null) {
			return;
		}
		if (keep && !buffer.isBlank()) {
			renaming.name = buffer.strip();
		}
		renaming = null;
		stopTyping();
		module.store().save();
	}

	// ---- input ----

	@Override
	protected boolean onKey(int key, int modifiers) {
		if (renaming != null) {
			switch (key) {
				case Keyboard.KEY_RETURN -> stopRename(true);
				case Keyboard.KEY_ESCAPE -> stopRename(false);
				case Keyboard.KEY_BACK -> {
					if (!buffer.isEmpty()) {
						buffer = buffer.substring(0, buffer.offsetByCodePoints(buffer.length(), -1));
					}
				}
				default -> {
				}
			}
			return true;
		}
		Waypoint p = current();
		switch (key) {
			case Keyboard.KEY_ESCAPE -> onClose();
			case Keyboard.KEY_UP -> selected = list.isEmpty() ? 0 : Math.floorMod(selected - 1, list.size());
			case Keyboard.KEY_DOWN -> selected = list.isEmpty() ? 0 : Math.floorMod(selected + 1, list.size());
			case Keyboard.KEY_RETURN -> {
				if (p != null) {
					startRename(p);
				}
			}
			case Keyboard.KEY_SPACE -> {
				if (p != null) {
					p.visible = !p.visible;
				}
			}
			case Keyboard.KEY_C -> {
				if (p != null) {
					p.color = Math.floorMod(p.color + 1, Waypoint.COLORS.length);
				}
			}
			case Keyboard.KEY_D, Keyboard.KEY_DELETE -> {
				if (p != null) {
					list.remove(p);
				}
			}
			case Keyboard.KEY_A -> {
				Waypoint added = module.addHere(MinecraftClient.getInstance(), "waypoint " + (list.size() + 1));
				if (added != null) {
					selected = list.indexOf(added);
					swallowUntil = System.currentTimeMillis() + 150;
					startRename(added);
				}
			}
			default -> {
				return false;
			}
		}
		module.store().save();
		return true;
	}

	@Override
	protected boolean onChar(String c, boolean allowedInChat) {
		if (renaming == null) {
			return false;
		}
		if (System.currentTimeMillis() < swallowUntil && (c.equalsIgnoreCase("a") || c.equalsIgnoreCase("b"))) {
			swallowUntil = 0;
			return true;
		}
		swallowUntil = 0;
		if (allowedInChat && buffer.length() < 32) {
			buffer += c;
		}
		return true;
	}

	@Override
	protected boolean onClick(double x, double y, int button) {
		return button == 0 && hits.click(x, y);
	}

	@Override
	public boolean mouseScrolled(double x, double y, double horizontal, double vertical) {
		if (vertical != 0 && !list.isEmpty()) {
			selected = Math.floorMod(selected + (vertical > 0 ? -1 : 1), list.size());
			return true;
		}
		return false;
	}

	// ---- drawing ----

	@Override
	protected void drawBackground(Gfx g, int mouseX, int mouseY, float delta) {
		vanillaBackground(g, mouseX, mouseY, delta);
		g.fill(x0, y0, x0 + w, y0 + h, Theme.WINDOW);
		Term.outline(g, x0, y0, w, h, Theme.accent());
	}

	@Override
	protected void draw(Gfx g, int mouseX, int mouseY, float delta) {
		hits.clear();
		int accent = Theme.accent();
		MinecraftClient mc = MinecraftClient.getInstance();
		new Term.Line().add("armstrong", accent).add("@client: ~/waypoints", Theme.MUTED).draw(g, x0 + 6, y0 + 3);
		String right = list.size() + " here";
		Term.draw(g, right, x0 + w - 6 - Term.width(right), y0 + 3, Theme.MUTED);
		g.fill(x0 + 1, y0 + BAR, x0 + w - 1, y0 + BAR + 1, Theme.BORDER);

		int x = x0 + 8, y = y0 + BAR + 6, cw = w - 16;
		Term.draw(g, "  " + Term.pad("NAME", 18) + Term.pad("XYZ", 20) + "DIM", x, y, Theme.MUTED);
		y += ROW - 1;
		g.fill(x, y, x + cw, y + 1, Theme.RULE);
		y += 3;

		int fits = Math.max(1, (y0 + h - BAR - 4 - y) / ROW);
		if (selected < scroll) {
			scroll = selected;
		} else if (selected >= scroll + fits) {
			scroll = selected - fits + 1;
		}
		scroll = Math.clamp(scroll, 0, Math.max(0, list.size() - fits));
		if (list.isEmpty()) {
			Term.draw(g, mc.player == null ? "  join a world to add waypoints" : "  none yet · press a to add one here", x, y, Theme.MUTED);
		}
		for (int i = scroll; i < list.size() && i < scroll + fits; i++) {
			Waypoint p = list.get(i);
			boolean sel = i == selected;
			if (sel) {
				g.fill(x, y - 2, x + cw, y + ROW - 2, Theme.accentDim());
			}
			Term.Line line = new Term.Line().add(sel ? "> " : "  ", Theme.AMBER);
			if (p == renaming) {
				String shown = buffer + ((System.currentTimeMillis() / 500) % 2 == 0 ? "_" : " ");
				line.add(Term.pad(shown, 18), Theme.BRIGHT);
			} else {
				line.add("◆ ", p.visible ? p.argb() : Theme.TRACK)
						.add(Term.pad(p.name, 16), p.visible ? (sel ? Theme.BRIGHT : Theme.TEXT) : Theme.MUTED);
			}
			String dim = p.dimension.substring(p.dimension.indexOf(':') + 1);
			line.add(Term.pad(p.x + " " + p.y + " " + p.z, 20), Theme.TEXT).add(dim, Theme.MUTED);
			line.draw(g, x, y);
			final int index = i;
			hits.add(x, y - 2, cw, ROW, () -> {
				if (selected == index && renaming == null) {
					startRename(list.get(index));
				}
				selected = index;
			});
			y += ROW;
		}

		int fy = y0 + h - BAR;
		g.fill(x0 + 1, fy, x0 + w - 1, fy + 1, Theme.BORDER);
		Term.Line hints = renaming != null
				? new Term.Line().add("type", Theme.AMBER).add(" name  ", Theme.MUTED).add("enter", Theme.AMBER).add(" save  ", Theme.MUTED)
						.add("esc", Theme.AMBER).add(" cancel", Theme.MUTED)
				: new Term.Line().add("a", Theme.AMBER).add(" add here  ", Theme.MUTED).add("enter", Theme.AMBER).add(" rename  ", Theme.MUTED)
						.add("space", Theme.AMBER).add(" show  ", Theme.MUTED).add("c", Theme.AMBER).add(" colour  ", Theme.MUTED)
						.add("d", Theme.AMBER).add(" delete", Theme.MUTED);
		hints.draw(g, x0 + 6, fy + 3);
	}

	@Override
	public void onClose() {
		Mc.open(null);
	}
}
