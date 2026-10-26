package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.fabricmc.fabric.api.client.rendering.v1.hud.HudElement;
import net.minecraft.client.DeltaTracker;
import net.minecraft.client.gui.GuiGraphicsExtractor;

/**
 * Scales or hides parts of the vanilla HUD: scoreboard, boss bar, titles.
 * Each is drawn by the game as usual, just scaled around its own anchor.
 */
public final class HudTweaks extends Module {
	private final Setting.Bool scoreboard = add(new Setting.Bool("scoreboard", "show_scoreboard", true));
	private final Setting.Number scoreboardScale = add(new Setting.Number("scoreboard_scale", "scoreboard_scale", 100, 50, 150, 5, "%"));
	private final Setting.Bool bossBar = add(new Setting.Bool("boss_bar", "show_boss_bar", true));
	private final Setting.Number bossBarScale = add(new Setting.Number("boss_bar_scale", "boss_bar_scale", 100, 50, 150, 5, "%"));
	private final Setting.Bool titles = add(new Setting.Bool("titles", "show_titles", true));
	private final Setting.Number titleScale = add(new Setting.Number("title_scale", "title_scale", 100, 30, 150, 5, "%"));

	public HudTweaks() {
		super("hud_tweaks", "HUD Tweaks", "Scale or hide the scoreboard, boss bar and titles.", Category.HUD, false);
	}

	/** Where each element scales from. */
	private enum Anchor { RIGHT_MIDDLE, TOP_CENTER, CENTER }

	public HudElement scoreboard(HudElement vanilla) {
		return wrap(vanilla, scoreboard, scoreboardScale, Anchor.RIGHT_MIDDLE);
	}

	public HudElement bossBar(HudElement vanilla) {
		return wrap(vanilla, bossBar, bossBarScale, Anchor.TOP_CENTER);
	}

	public HudElement titles(HudElement vanilla) {
		return wrap(vanilla, titles, titleScale, Anchor.CENTER);
	}

	private HudElement wrap(HudElement vanilla, Setting.Bool show, Setting.Number scale, Anchor anchor) {
		return (GuiGraphicsExtractor g, DeltaTracker delta) -> {
			if (!isEnabled()) {
				vanilla.extractRenderState(g, delta);
				return;
			}
			if (!show.get()) {
				return;
			}
			float s = (float) (scale.get() / 100.0);
			if (s == 1f) {
				vanilla.extractRenderState(g, delta);
				return;
			}
			float ax = switch (anchor) {
				case RIGHT_MIDDLE -> g.guiWidth();
				default -> g.guiWidth() / 2f;
			};
			float ay = switch (anchor) {
				case RIGHT_MIDDLE, CENTER -> g.guiHeight() / 2f;
				case TOP_CENTER -> 0f;
			};
			g.pose().pushMatrix();
			g.pose().translate(ax, ay);
			g.pose().scale(s, s);
			g.pose().translate(-ax, -ay);
			vanilla.extractRenderState(g, delta);
			g.pose().popMatrix();
		};
	}
}
