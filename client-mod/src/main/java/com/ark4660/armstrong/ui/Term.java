package com.ark4660.armstrong.ui;

import java.util.ArrayList;
import java.util.List;

import com.ark4660.armstrong.ArmstrongClient;

import net.minecraft.client.Minecraft;
import net.minecraft.client.gui.Font;
import net.minecraft.client.gui.GuiGraphicsExtractor;
import net.minecraft.network.chat.Component;
import net.minecraft.network.chat.FontDescription;
import net.minecraft.network.chat.MutableComponent;

/** Drawing helpers for terminal-style text: spans of colored monospace text and panels. */
public final class Term {
	public static final FontDescription MONO = new FontDescription.Resource(ArmstrongClient.id("mono"));
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
		g.text(font(), text(s), x, y, color, false);
	}

	/** A run of text in one color. */
	public record Span(String text, int color) {}

	/** A line made of colored spans, built fluently: new Line().add("fps ", MUTED).add("241", accent). */
	public static final class Line {
		final List<Span> spans = new ArrayList<>();

		public Line add(String text, int color) {
			spans.add(new Span(text, color));
			return this;
		}

		public Line append(Line other) {
			spans.addAll(other.spans);
			return this;
		}

		public int width() {
			int w = 0;
			for (Span s : spans) {
				w += Term.width(s.text);
			}
			return w;
		}

		public int draw(GuiGraphicsExtractor g, int x, int y) {
			for (Span s : spans) {
				Term.draw(g, s.text, x, y, s.color);
				x += Term.width(s.text);
			}
			return x;
		}
	}

	/** A HUD/menu box: translucent fill with a 1px border. */
	public static void panel(GuiGraphicsExtractor g, int x, int y, int w, int h, int fill, int border) {
		g.fill(x, y, x + w, y + h, fill);
		g.outline(x, y, w, h, border);
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
