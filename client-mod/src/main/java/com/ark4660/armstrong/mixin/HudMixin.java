package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;

import net.minecraft.client.gui.Gui;

/**
 * Before 26.x Fabric can't replace vanilla HUD elements, so the effect icons,
 * crosshair, scoreboard and titles go through HudHooks from here. (26.x does
 * the same with Fabric's HUD registry, in ArmstrongClient; this is empty there.)
 */
@Mixin(Gui.class)
public class HudMixin {
	//? if <26 {
	/*@com.llamalad7.mixinextras.injector.wrapmethod.WrapMethod(method = "renderEffects")
	private void armstrong$effects(net.minecraft.client.gui.GuiGraphicsExtractor g, net.minecraft.client.DeltaTracker delta,
			com.llamalad7.mixinextras.injector.wrapoperation.Operation<Void> vanilla) {
		com.ark4660.armstrong.HudHooks.effects(g, () -> vanilla.call(g, delta));
	}

	@com.llamalad7.mixinextras.injector.wrapmethod.WrapMethod(method = "renderCrosshair")
	private void armstrong$crosshair(net.minecraft.client.gui.GuiGraphicsExtractor g, net.minecraft.client.DeltaTracker delta,
			com.llamalad7.mixinextras.injector.wrapoperation.Operation<Void> vanilla) {
		com.ark4660.armstrong.HudHooks.crosshair(g, () -> vanilla.call(g, delta));
	}

	@com.llamalad7.mixinextras.injector.wrapmethod.WrapMethod(method = "renderScoreboardSidebar")
	private void armstrong$scoreboard(net.minecraft.client.gui.GuiGraphicsExtractor g, net.minecraft.client.DeltaTracker delta,
			com.llamalad7.mixinextras.injector.wrapoperation.Operation<Void> vanilla) {
		com.ark4660.armstrong.HudHooks.scoreboard(g, () -> vanilla.call(g, delta));
	}

	@com.llamalad7.mixinextras.injector.wrapmethod.WrapMethod(method = "renderTitle")
	private void armstrong$titles(net.minecraft.client.gui.GuiGraphicsExtractor g, net.minecraft.client.DeltaTracker delta,
			com.llamalad7.mixinextras.injector.wrapoperation.Operation<Void> vanilla) {
		com.ark4660.armstrong.HudHooks.titles(g, () -> vanilla.call(g, delta));
	}
	*///?}
}
