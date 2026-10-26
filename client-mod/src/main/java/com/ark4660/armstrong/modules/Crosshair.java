package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Theme;

import net.minecraft.client.gui.GuiGraphicsExtractor;

/** A custom crosshair drawn in place of the vanilla one. */
public final class Crosshair extends Module {
	private final Setting.Choice style = add(new Setting.Choice("style", "style", "Cross", "Cross", "Dot", "Circle", "T", "Cross + dot"));
	private final Setting.Choice color = add(new Setting.Choice("color", "color", "White", "White", "Accent", "Red", "Yellow", "Cyan"));
	private final Setting.Number size = add(new Setting.Number("size", "length", 4, 1, 12, 1, "px"));
	private final Setting.Number gap = add(new Setting.Number("gap", "gap", 2, 0, 10, 1, "px"));
	private final Setting.Number thickness = add(new Setting.Number("thickness", "thickness", 1, 1, 4, 1, "px"));
	private final Setting.Bool outline = add(new Setting.Bool("outline", "outline", true));

	public Crosshair() {
		super("crosshair", "Crosshair", "Your own crosshair: style, color, size and gap.", Category.HUD, false);
	}

	private int argb() {
		return switch (color.get()) {
			case "Accent" -> Theme.accent();
			case "Red" -> 0xFFFF5555;
			case "Yellow" -> 0xFFFFE066;
			case "Cyan" -> Theme.CYAN;
			default -> 0xFFFFFFFF;
		};
	}

	/** Draws the crosshair centred on the screen. */
	public void draw(GuiGraphicsExtractor g) {
		int cx = g.guiWidth() / 2, cy = g.guiHeight() / 2;
		int len = (int) Math.round(size.get());
		int gp = (int) Math.round(gap.get());
		int t = (int) Math.round(thickness.get());
		int lo = -t / 2, hi = lo + t; // thickness around the centre line
		int c = argb();
		String s = style.get();
		boolean cross = s.equals("Cross") || s.equals("Cross + dot") || s.equals("T");
		if (cross) {
			if (!s.equals("T")) {
				bar(g, cx + lo, cy - gp - len, cx + hi, cy - gp, c); // up
			}
			bar(g, cx + lo, cy + gp, cx + hi, cy + gp + len, c);     // down
			bar(g, cx - gp - len, cy + lo, cx - gp, cy + hi, c);     // left
			bar(g, cx + gp, cy + lo, cx + gp + len, cy + hi, c);     // right
		}
		if (s.equals("Dot") || s.equals("Cross + dot")) {
			bar(g, cx + lo, cy + lo, cx + hi, cy + hi, c);
		}
		if (s.equals("Circle")) {
			int r = gp + len;
			bar(g, cx - r, cy - r, cx + r + 1, cy - r + t, c);
			bar(g, cx - r, cy + r + 1 - t, cx + r + 1, cy + r + 1, c);
			bar(g, cx - r, cy - r, cx - r + t, cy + r + 1, c);
			bar(g, cx + r + 1 - t, cy - r, cx + r + 1, cy + r + 1, c);
		}
	}

	private void bar(GuiGraphicsExtractor g, int x1, int y1, int x2, int y2, int color) {
		if (outline.get()) {
			g.fill(x1 - 1, y1 - 1, x2 + 1, y2 + 1, 0xC0000000);
		}
		g.fill(x1, y1, x2, y2, color);
	}
}
