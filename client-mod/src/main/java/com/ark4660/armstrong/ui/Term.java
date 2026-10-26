package com.ark4660.armstrong.ui;

import java.util.ArrayList;
import java.util.List;

import com.ark4660.armstrong.ArmstrongClient;

import net.minecraft.client.Minecraft;
import net.minecraft.client.gui.Font;
import net.minecraft.client.gui.GuiGraphicsExtractor;
import net.minecraft.network.chat.Component;
//? if >=26
import net.minecraft.network.chat.FontDescription;
import net.minecraft.network.chat.MutableComponent;

/** Drawing helpers for terminal-style text: spans of colored monospace text and panels. */
public final class Term {
	//? if >=26 {
	public static final FontDescription MONO = new FontDescription.Resource(ArmstrongClient.id("mono"));
	//?} else {
	/*public static final net.minecraft.resources.Identifier MONO = ArmstrongClient.id("mono");
	*///?}
	public static final int LINE = 10; // line height in GUI pixels

	private Term() {}

	public static Font font() {
		return Minecraft.getInstance().font;
	}

	/** Text in the terminal font (or the game's font if the player turned it off). */
	public static MutableComponent text(String s) {
		MutableComponent c = Component.literal(s);
		return Theme.monospace() ? c.withStyle(style -> style.withFont(MONO)) : c;
	}

	public static int width(String s) {
		return font().width(text(s));
	}

	/** Width of one character cell, for laying out columns. */
	public static int cell() {
		return width("0");
	}

	public static void draw(GuiGraphicsExtractor g, String s, int x, int y, int color) {
		//? if >=26 {
		g.text(font(), text(s), x, y, color, false);
		//?} else {
		/*g.drawString(font(), text(s), x, y, color, false);
		*///?}
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

		/**
		 * A solid bar `cells` characters wide, drawn as a rectangle: a block
		 * glyph's height varies by version and font, so rows could merge.
		 */
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

		public int draw(GuiGraphicsExtractor g, int x, int y) {
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
	public static void panel(GuiGraphicsExtractor g, int x, int y, int w, int h, int fill, int border) {
		g.fill(x, y, x + w, y + h, fill);
		outline(g, x, y, w, h, border);
	}

	/** A 1px rectangle outline. */
	public static void outline(GuiGraphicsExtractor g, int x, int y, int w, int h, int color) {
		//? if >=26 {
		g.outline(x, y, w, h, color);
		//?} else {
		/*g.renderOutline(x, y, w, h, color);
		*///?}
	}

	/** Shortens text with … so it's at most maxWidth pixels wide. */
	public static String fit(String s, int maxWidth) {
		if (width(s) <= maxWidth) {
			return s;
		}
		while (!s.isEmpty() && width(s + "…") > maxWidth) {
			s = s.substring(0, s.offsetByCodePoints(s.length(), -1));
		}
		return s.stripTrailing() + "…";
	}

	/**
	 * Word-wraps text to maxWidth pixels, at most maxLines lines; the last
	 * line ends with … if there was more.
	 */
	public static List<String> wrap(String s, int maxWidth, int maxLines) {
		List<String> out = new ArrayList<>();
		String line = "";
		String[] words = s.split(" ");
		for (int i = 0; i < words.length; i++) {
			String next = line.isEmpty() ? words[i] : line + " " + words[i];
			if (width(next) <= maxWidth || line.isEmpty()) {
				line = next;
				continue;
			}
			if (out.size() == maxLines - 1) {
				out.add(fit(String.join(" ", line, String.join(" ", java.util.Arrays.copyOfRange(words, i, words.length))), maxWidth));
				return out;
			}
			out.add(line);
			line = words[i];
		}
		if (!line.isEmpty()) {
			out.add(fit(line, maxWidth));
		}
		return out;
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
