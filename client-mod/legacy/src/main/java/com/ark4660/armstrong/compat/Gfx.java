package com.ark4660.armstrong.compat;

import com.mojang.blaze3d.platform.GlStateManager;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.gui.DrawableHelper;
import net.minecraft.client.util.Window;

/**
 * 1.8.9 has no GuiGraphics: drawing is immediate GL. This stands in for it
 * with the calls the shared code's ports use (fill, outline, text, size), so
 * the ported classes read like the modern ones.
 */
public final class Gfx {
	private final int width, height;

	public Gfx() {
		Window w = new Window(MinecraftClient.getInstance());
		width = w.getWidth();
		height = w.getHeight();
	}

	/** Scaled GUI width, like GuiGraphics.guiWidth(). */
	public int guiWidth() {
		return width;
	}

	public int guiHeight() {
		return height;
	}

	public void fill(int x1, int y1, int x2, int y2, int color) {
		DrawableHelper.fill(x1, y1, x2, y2, color);
	}

	/** A 1px rectangle outline. */
	public void outline(int x, int y, int w, int h, int color) {
		fill(x, y, x + w, y + 1, color);
		fill(x, y + h - 1, x + w, y + h, color);
		fill(x, y + 1, x + 1, y + h - 1, color);
		fill(x + w - 1, y + 1, x + w, y + h - 1, color);
	}

	/** Text without shadow at x, y. */
	public void text(String s, float x, float y, int color) {
		MinecraftClient.getInstance().textRenderer.draw(s, x, y, color, false);
		GlStateManager.color(1f, 1f, 1f, 1f);
	}

	public void push() {
		GlStateManager.pushMatrix();
	}

	public void pop() {
		GlStateManager.popMatrix();
	}

	public void translate(float x, float y) {
		GlStateManager.translate(x, y, 0f);
	}

	public void scale(float s) {
		GlStateManager.scale(s, s, 1f);
	}
}
