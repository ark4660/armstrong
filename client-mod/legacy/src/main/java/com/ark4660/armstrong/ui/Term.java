package com.ark4660.armstrong.ui;

import java.util.ArrayList;
import java.util.List;

import com.ark4660.armstrong.compat.Gfx;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.font.TextRenderer;

/**
 * Drawing helpers for terminal-style text: spans of colored text and panels.
 * 1.8.9 can't load the TrueType terminal font, so monospace is done by hand:
 * every character is centred in a fixed-width cell of the game's font.
 */
public final class Term {
	public static final int LINE = 10; // line height in GUI pixels
	private static final int CELL = 6;  // monospace cell width

	private Term() {}

	public static TextRenderer font() {
		return MinecraftClient.getInstance().textRenderer;
	}

	public static int width(String s) {
		return Theme.monospace() ? s.codePointCount(0, s.length()) * CELL : font().getStringWidth(s);
	}

	/** Width of one character cell, for laying out columns. */
	public static int cell() {
		return Theme.monospace() ? CELL : font().getStringWidth("0");
	}

	public static void draw(Gfx g, String s, int x, int y, int color) {
		if (!Theme.monospace()) {
			g.text(s, x, y, color);
			return;
		}
		TextRenderer f = font();
		for (int i = 0; i < s.length(); ) {
			int cp = s.codePointAt(i);
			String c = new String(Character.toChars(cp));
			if (cp != ' ') {
				// Glyph widths include 1px spacing on the right.
				g.text(c, x + (CELL - (f.getStringWidth(c) - 1)) / 2f, y, color);
			}
			x += CELL;
			i += Character.charCount(cp);
		}
	}

	/** A run of text in one color, or (bar) a solid block `text.length()` cells wide. */
	public record Span(String text, int color, boolean bar) {
		int width() {
			return bar ? text.length() * cell() : Term.width(text);
		}
	}

	/** A line made of colored spans, built fluently: new Line().add("fps ", MUTED).add("241", accent). */
	public static final class Line {
		final List<Span> spans = new ArrayList<>();

		public Line add(String text, int color) {
			spans.add(new Span(text, color, false));
			return this;
		}

		/** A solid bar `cells` characters wide, drawn as a rectangle. */
		public Line bar(int cells, int color) {
			if (cells > 0) {
				spans.add(new Span(" ".repeat(cells), color, true));
			}
			return this;
		}

		public Line append(Line other) {
			spans.addAll(other.spans);
			return this;
		}

		public int width() {
			int w = 0;
			for (Span s : spans) {
				w += s.width();
			}
			return w;
		}

		public int draw(Gfx g, int x, int y) {
			for (Span s : spans) {
				if (s.bar) {
					g.fill(x, y, x + s.width(), y + 7, s.color);
				} else {
					Term.draw(g, s.text, x, y, s.color);
				}
				x += s.width();
			}
			return x;
		}
	}

	/** A HUD/menu box: translucent fill with a 1px border. */
	public static void panel(Gfx g, int x, int y, int w, int h, int fill, int border) {
		g.fill(x, y, x + w, y + h, fill);
		outline(g, x, y, w, h, border);
	}

	/** A 1px rectangle outline. */
	public static void outline(Gfx g, int x, int y, int w, int h, int color) {
		g.outline(x, y, w, h, color);
	}

	/** Pads with spaces to n characters (monospace columns). */
	public static String pad(String s, int n) {
		return s.length() >= n ? s : s + " ".repeat(n - s.length());
	}

	/** An ASCII bar like [####······] for a value from 0 to 1. */
	public static Line bar(double fraction, int cells, int fillColor) {
		int filled = (int) Math.round(Math.clamp(fraction, 0, 1) * cells);
		return new Line().add("[", fillColor).add("#".repeat(filled), fillColor)
				.add("·".repeat(cells - filled), Theme.TRACK).add("]", fillColor);
	}
}
