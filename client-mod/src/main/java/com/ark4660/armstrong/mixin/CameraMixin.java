package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.Zoom;

import net.minecraft.client.Camera;

/** Narrows the world field of view while Zoom is active (the hand and HUD use a separate FOV). */
@Mixin(Camera.class)
public class CameraMixin {
	@Inject(method = "getFov", at = @At("RETURN"), cancellable = true)
	private void armstrong$zoom(CallbackInfoReturnable<Float> cir) {
		Zoom zoom = ArmstrongClient.zoom();
		if (zoom != null && zoom.active()) {
			cir.setReturnValue(cir.getReturnValueF() / zoom.divisor());
		}
	}
}
