package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.compat.Gfx;
import com.ark4660.armstrong.modules.Crosshair;
import com.llamalad7.mixinextras.injector.ModifyExpressionValue;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.gui.hud.InGameHud;

/** The custom crosshair, drawn where the game would draw its own (first person). */
@Mixin(InGameHud.class)
public class InGameHudMixin {
	@ModifyExpressionValue(method = "render", at = @At(value = "INVOKE", target = "Lnet/minecraft/client/gui/hud/InGameHud;showCrosshair()Z"))
	private boolean armstrong$crosshair(boolean show) {
		Crosshair crosshair = ArmstrongClient.active(Crosshair.class);
		if (crosshair == null) {
			return show;
		}
		if (show && MinecraftClient.getInstance().options.perspective == 0) {
			crosshair.draw(new Gfx());
		}
		return false;
	}
}
