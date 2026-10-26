package com.ark4660.armstrong;

import com.ark4660.armstrong.modules.Crosshair;
import com.ark4660.armstrong.modules.EffectsHud;
import com.ark4660.armstrong.modules.HudTweaks;

import net.minecraft.client.Minecraft;
import net.minecraft.client.gui.GuiGraphicsExtractor;

/**
 * Armstrong's changes to vanilla HUD elements. Each hook gets the game's own
 * drawing as `vanilla` and decides whether, and how, to run it. 26.x calls
 * these through Fabric's HUD registry; older versions through mixins on Gui.
 */
public final class HudHooks {
	private HudHooks() {}

	/** The effects panel can replace the game's effect icons. */
	public static void effects(GuiGraphicsExtractor g, Runnable vanilla) {
		EffectsHud effects = ArmstrongClient.active(EffectsHud.class);
		if (effects == null || !effects.hidesVanilla()) {
			vanilla.run();
		}
	}

	/** The custom crosshair, shown where the game would show its own (first person). */
	public static void crosshair(GuiGraphicsExtractor g, Runnable vanilla) {
		Crosshair crosshair = ArmstrongClient.active(Crosshair.class);
		if (crosshair == null) {
			vanilla.run();
		} else if (Minecraft.getInstance().options.getCameraType().isFirstPerson()) {
			crosshair.draw(g);
		}
	}

	public static void scoreboard(GuiGraphicsExtractor g, Runnable vanilla) {
		tweaks(vanilla, t -> t.scoreboard(g, vanilla));
	}

	public static void bossBar(GuiGraphicsExtractor g, Runnable vanilla) {
		tweaks(vanilla, t -> t.bossBar(g, vanilla));
	}

	public static void titles(GuiGraphicsExtractor g, Runnable vanilla) {
		tweaks(vanilla, t -> t.titles(g, vanilla));
	}

	private static void tweaks(Runnable vanilla, java.util.function.Consumer<HudTweaks> apply) {
		HudTweaks t = ArmstrongClient.modules() == null ? null : ArmstrongClient.modules().get(HudTweaks.class);
		if (t == null) {
			vanilla.run();
		} else {
			apply.accept(t);
		}
	}
}
