package com.ark4660.armstrong.gui;

import java.util.ArrayList;
import java.util.List;

import com.ark4660.armstrong.compat.CompatScreen;
import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.module.HudModule;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.ModuleManager;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;
import com.mojang.blaze3d.platform.InputConstants;

import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;
import net.minecraft.client.gui.GuiGraphicsExtractor;
import net.minecraft.client.gui.screens.Screen;
import net.minecraft.network.chat.Component;

/**
 * The HUD editor: drag panels anywhere, with snapping to the screen edges,
 * the centre lines and other panels. Scroll over a panel to resize it,
 * arrows nudge the selected one, r resets it to its corner.
 */
public final class HudEditorScreen extends CompatScreen {
	private static final int SNAP = 4; // GUI pixels

	private final ModuleManager modules;
	private final KeyMapping menuKey;
	private final Screen parent;

	private HudModule selected;
	private HudModule dragging;
	private double grabX, grabY; // where in the panel it was picked up
	private Integer guideX, guideY; // snap lines to draw while dragging

	public HudEditorScreen(ModuleManager modules, KeyMapping menuKey, Screen parent) {
		super(Component.literal("HUD editor"));
		this.modules = modules;
		this.menuKey = menuKey;
		this.parent = parent;
	}

	@Override
	protected void init() {
		modules.setEditorOpen(true);
	}

	@Override
	public void removed() {
		modules.setEditorOpen(false);
		modules.save();
		super.removed();
	}

	@Override
	public boolean isPauseScreen() {
		return false;
	}

	private List<HudModule> panels() {
		List<HudModule> out = new ArrayList<>();
		for (Module m : modules.all()) {
			if (m instanceof HudModule hud && hud.isActive() && hud.lastW > 0) {
				out.add(hud);
			}
		}
		return out;
	}

	/** The topmost panel under the point (drawn last = on top). */
	private HudModule at(double x, double y) {
		List<HudModule> list = panels();
		for (int i = list.size() - 1; i >= 0; i--) {
			HudModule p = list.get(i);
			if (x >= p.lastX && x < p.lastX + p.lastW && y >= p.lastY && y < p.lastY + p.lastH) {
				return p;
			}
		}
		return null;
	}

	// ---- input ----

	@Override
	protected boolean onClick(double x, double y, int button) {
		if (button != InputConstants.MOUSE_BUTTON_LEFT) {
			return false;
		}
		selected = at(x, y);
		dragging = selected;
		if (dragging != null) {
			grabX = x - dragging.lastX;
			grabY = y - dragging.lastY;
		}
		return true;
	}

	@Override
	protected boolean onDrag(double x, double y, int button, double dx, double dy) {
		if (dragging == null) {
			return false;
		}
		int w = dragging.lastW, h = dragging.lastH;
		int nx = (int) Math.round(x - grabX);
		int ny = (int) Math.round(y - grabY);
		guideX = null;
		guideY = null;
		if (!Mc.altDown()) { // alt: place freely
			nx = snap(nx, w, width, true);
			ny = snap(ny, h, height, false);
		}
		nx = Math.clamp(nx, 0, Math.max(0, width - w));
		ny = Math.clamp(ny, 0, Math.max(0, height - h));
		dragging.place(nx, ny, width, height);
		return true;
	}

	@Override
	protected boolean onRelease(double x, double y, int button) {
		if (dragging == null) {
			return false;
		}
		dragging = null;
		guideX = null;
		guideY = null;
		return true;
	}

	/**
	 * Snaps one axis: the panel's start, centre or end to the screen's edges
	 * and centre and to other panels' edges, whichever is within SNAP.
	 */
	private int snap(int pos, int size, int screen, boolean horizontal) {
		List<Integer> targets = new ArrayList<>(List.of(0, screen / 2, screen));
		for (HudModule p : panels()) {
			if (p != dragging) {
				int start = horizontal ? p.lastX : p.lastY;
				int len = horizontal ? p.lastW : p.lastH;
				targets.add(start);
				targets.add(start + len);
			}
		}
		int best = SNAP + 1, out = pos;
		Integer line = null;
		int[] offsets = {0, size / 2, size}; // start, centre, end of the panel
		for (int t : targets) {
			for (int off : offsets) {
				int d = Math.abs(pos + off - t);
				// The screen edges snap flush; a centre only snaps to a centre line.
				if (d < best && (off != size / 2 || t == screen / 2)) {
					best = d;
					out = t - off;
					line = t;
				}
			}
		}
		if (line != null && line > 0 && line < screen) {
			if (horizontal) {
				guideX = line;
			} else {
				guideY = line;
			}
		}
		return out;
	}

	@Override
	public boolean mouseScrolled(double x, double y, double horizontal, double vertical) {
		HudModule p = at(x, y);
		if (p == null) {
			p = selected;
		}
		if (p == null || vertical == 0) {
			return false;
		}
		selected = p;
		// Resize around the panel's centre, so it grows in place. The new
		// size follows from the scale change (it's measured on the next frame).
		int cx = p.lastX + p.lastW / 2, cy = p.lastY + p.lastH / 2;
		float before = p.scale();
		p.rescale(vertical > 0 ? 1 : -1);
		float ratio = p.scale() / before;
		p.lastW = Math.round(p.lastW * ratio);
		p.lastH = Math.round(p.lastH * ratio);
		p.place(Math.clamp(cx - p.lastW / 2, 0, Math.max(0, width - p.lastW)),
				Math.clamp(cy - p.lastH / 2, 0, Math.max(0, height - p.lastH)), width, height);
		return true;
	}

	@Override
	protected boolean onKey(int key, int modifiers) {
		if (key == InputConstants.KEY_ESCAPE || Mc.isBound(menuKey, key)) {
			onClose();
			return true;
		}
		if (selected == null) {
			if (key == InputConstants.KEY_TAB) {
				List<HudModule> list = panels();
				selected = list.isEmpty() ? null : list.get(0);
				return true;
			}
			return false;
		}
		int step = (modifiers & 1) != 0 ? 10 : 1; // shift: 10 px
		switch (key) {
			case InputConstants.KEY_LEFT -> selected.place(Math.max(0, selected.lastX - step), selected.lastY, width, height);
			case InputConstants.KEY_RIGHT -> selected.place(Math.min(width - selected.lastW, selected.lastX + step), selected.lastY, width, height);
			case InputConstants.KEY_UP -> selected.place(selected.lastX, Math.max(0, selected.lastY - step), width, height);
			case InputConstants.KEY_DOWN -> selected.place(selected.lastX, Math.min(height - selected.lastH, selected.lastY + step), width, height);
			case InputConstants.KEY_R -> selected.resetPlacement();
			case InputConstants.KEY_DELETE -> {
				selected.setEnabled(false); // hide it; switch it back on in the menu
				selected = null;
			}
			case InputConstants.KEY_TAB -> {
				List<HudModule> list = panels();
				if (!list.isEmpty()) {
					int i = list.indexOf(selected);
					selected = list.get(Math.floorMod(i + ((modifiers & 1) != 0 ? -1 : 1), list.size()));
				}
			}
			default -> {
				return false;
			}
		}
		return true;
	}

	@Override
	public void onClose() {
		Mc.open(parent);
	}

	// ---- drawing ----

	@Override
	protected void drawBackground(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		// No blur or dimming: the panels are placed against the real game view.
		g.fill(width / 2, 0, width / 2 + 1, height, 0x30FFFFFF);
		g.fill(0, height / 2, width, height / 2 + 1, 0x30FFFFFF);
	}

	@Override
	protected void draw(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		modules.renderHud(g, Minecraft.getInstance(), true);
		int accent = Theme.accent();
		HudModule hover = dragging != null ? dragging : at(mouseX, mouseY);
		for (HudModule p : panels()) {
			boolean sel = p == selected;
			int color = sel ? accent : p == hover ? Theme.AMBER : 0x80FFFFFF;
			Term.outline(g, p.lastX - 1, p.lastY - 1, p.lastW + 2, p.lastH + 2, color);
			if (sel || p == hover) {
				String label = p.id + " · " + Math.round(p.scale() * 100) + "%";
				int ly = p.lastY >= 12 ? p.lastY - 11 : p.lastY + p.lastH + 2;
				int lw = Term.width(label) + 6;
				g.fill(p.lastX - 1, ly - 1, p.lastX - 1 + lw, ly + 9, Theme.WINDOW);
				Term.draw(g, label, p.lastX + 2, ly, color);
			}
		}
		if (guideX != null) {
			g.fill(guideX, 0, guideX + 1, height, accent);
		}
		if (guideY != null) {
			g.fill(0, guideY, width, guideY + 1, accent);
		}

		// Help bar.
		Term.Line help = new Term.Line()
				.add("drag", Theme.AMBER).add(" move  ", Theme.MUTED)
				.add("scroll", Theme.AMBER).add(" size  ", Theme.MUTED)
				.add("←↑↓→", Theme.AMBER).add(" nudge  ", Theme.MUTED)
				.add("tab", Theme.AMBER).add(" next  ", Theme.MUTED)
				.add("r", Theme.AMBER).add(" reset  ", Theme.MUTED)
				.add("del", Theme.AMBER).add(" hide  ", Theme.MUTED)
				.add("alt", Theme.AMBER).add(" no snap  ", Theme.MUTED)
				.add("esc", Theme.AMBER).add(" done", Theme.MUTED);
		if (help.width() + 12 > width) {
			help = new Term.Line().add("drag", Theme.AMBER).add(" move  ", Theme.MUTED)
					.add("scroll", Theme.AMBER).add(" size  ", Theme.MUTED)
					.add("esc", Theme.AMBER).add(" done", Theme.MUTED);
		}
		int hw = help.width() + 12;
		int hx = (width - hw) / 2, hy = height / 2 - 30;
		if (dragging == null) {
			Term.panel(g, hx, hy, hw, Term.LINE + 5, Theme.WINDOW, accent);
			help.draw(g, hx + 6, hy + 3);
			String title = "~/hud";
			Term.draw(g, title, (width - Term.width(title)) / 2, hy - 11, accent);
		}
	}
}
