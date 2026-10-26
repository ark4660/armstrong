package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;
import com.ark4660.armstrong.modules.Zoom;
import com.llamalad7.mixinextras.injector.ModifyExpressionValue;

import net.minecraft.client.render.GameRenderer;

/** Zoom narrows the world FOV; Fullbright raises the gamma the lightmap reads. */
@Mixin(GameRenderer.class)
public class GameRendererMixin {
	// changingFov is false for the hand, which keeps its own FOV.
	@Inject(method = "getFov", at = @At("RETURN"), cancellable = true)
	private void armstrong$zoom(float tickDelta, boolean changingFov, CallbackInfoReturnable<Float> cir) {
		Zoom zoom = ArmstrongClient.zoom();
		if (changingFov && zoom != null && zoom.active()) {
			cir.setReturnValue(cir.getReturnValueF() / zoom.divisor());
		}
	}

	@ModifyExpressionValue(method = "updateLightmap", at = @At(value = "FIELD", target = "Lnet/minecraft/client/option/GameOptions;gamma:F"))
	private float armstrong$fullbright(float gamma) {
		ViewModules.Fullbright fb = ArmstrongClient.active(ViewModules.Fullbright.class);
		return fb == null ? gamma : fb.gamma();
	}
}
