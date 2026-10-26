package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;

import net.minecraft.client.gui.components.BossHealthOverlay;

/** The boss bar through HudHooks before 26.x (see HudMixin); empty in 26.x. */
@Mixin(BossHealthOverlay.class)
public class BossBarMixin {
	//? if <26 {
	/*@com.llamalad7.mixinextras.injector.wrapmethod.WrapMethod(method = "render")
	private void armstrong$bossBar(net.minecraft.client.gui.GuiGraphicsExtractor g,
			com.llamalad7.mixinextras.injector.wrapoperation.Operation<Void> vanilla) {
		com.ark4660.armstrong.HudHooks.bossBar(g, () -> vanilla.call(g));
	}
	*///?}
}
