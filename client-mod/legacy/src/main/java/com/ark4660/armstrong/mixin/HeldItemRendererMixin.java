package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;
import com.mojang.blaze3d.platform.GlStateManager;

import net.minecraft.client.render.item.HeldItemRenderer;

/** Low Fire: shifts the burning overlay down while it's drawn. */
@Mixin(HeldItemRenderer.class)
public class HeldItemRendererMixin {
	@Inject(method = "renderFireOverlay", at = @At("HEAD"))
	private void armstrong$lowerFire(float tickDelta, CallbackInfo ci) {
		GlStateManager.pushMatrix();
		ViewModules.LowFire lf = ArmstrongClient.active(ViewModules.LowFire.class);
		if (lf != null) {
			GlStateManager.translate(0f, -lf.offset(), 0f);
		}
	}

	@Inject(method = "renderFireOverlay", at = @At("RETURN"))
	private void armstrong$restore(float tickDelta, CallbackInfo ci) {
		GlStateManager.popMatrix();
	}
}
