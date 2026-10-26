package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.input.ClickCounter;
import com.ark4660.armstrong.module.HudModule;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;
import net.minecraft.client.gui.GuiGraphicsExtractor;

/** W A S D, mouse buttons with clicks per second, and space: lit while held. */
public final class KeystrokesHud extends HudModule {
	private static final int KEY = 20, GAP = 2;

	private final Setting.Bool mouse = add(new Setting.Bool("mouse", "mouse_buttons", true));
	private final Setting.Bool space = add(new Setting.Bool("space", "space_bar", true));

	public KeystrokesHud() {
		super("keystrokes", "Keystrokes", "Shows movement keys and clicks as you press them.", false, "Bottom left");
	}

	@Override
	protected boolean ownBoxes() {
		return true;
	}

	@Override
	protected int width(Minecraft mc) {
		return 3 * KEY + 2 * GAP;
	}

	@Override
	protected int height(Minecraft mc) {
		int h = 2 * KEY + GAP;
		if (mouse.get()) {
			h += GAP + 16;
		}
		if (space.get()) {
			h += GAP + 11;
		}
		return h;
	}

	@Override
	protected void draw(GuiGraphicsExtractor g, Minecraft mc, int x, int y) {
		var o = mc.options;
		key(g, x + KEY + GAP, y, KEY, KEY, "W", o.keyUp);
		y += KEY + GAP;
		key(g, x, y, KEY, KEY, "A", o.keyLeft);
		key(g, x + KEY + GAP, y, KEY, KEY, "S", o.keyDown);
		key(g, x + 2 * (KEY + GAP), y, KEY, KEY, "D", o.keyRight);
		y += KEY + GAP;
		int full = 3 * KEY + 2 * GAP;
		if (mouse.get()) {
			int half = (full - GAP) / 2;
			key(g, x, y, half, 16, "LMB " + ClickCounter.left(), o.keyAttack);
			key(g, x + half + GAP, y, full - half - GAP, 16, "RMB " + ClickCounter.right(), o.keyUse);
			y += 16 + GAP;
		}
		if (space.get()) {
			key(g, x, y, full, 11, "───", o.keyJump);
		}
	}

	private void key(GuiGraphicsExtractor g, int x, int y, int w, int h, String label, KeyMapping mapping) {
		boolean down = mapping.isDown();
		int accent = Theme.accent();
		if (down) {
			g.fill(x, y, x + w, y + h, accent);
		} else if (background.get()) {
			Term.panel(g, x, y, w, h, Theme.PANEL, 0xFF2E4236);
		}
		int tx = x + (w - Term.width(label)) / 2;
		int ty = y + (h - 8) / 2;
		Term.draw(g, label, tx, ty, down ? Theme.ON_ACCENT : Theme.TEXT);
	}
}
