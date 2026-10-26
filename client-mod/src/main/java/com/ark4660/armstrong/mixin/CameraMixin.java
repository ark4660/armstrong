package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.modules.Zoom;

import net.minecraft.client.Camera;

/**
 * Narrows the world field of view while Zoom is active (the hand and HUD use a
 * separate FOV). Before 26.x the FOV lives in GameRenderer.getFov, whose
 * boolean is false for the hand.
 */
//? if >=26 {
@Mixin(Camera.class)
//?} else {
/*@Mixin(net.minecraft.client.renderer.GameRenderer.class)
*///?}
public class CameraMixin {
	//? if >=26 {
	@Inject(method = "getFov", at = @At("RETURN"), cancellable = true)
	private void armstrong$zoom(CallbackInfoReturnable<Float> cir) {
		Zoom zoom = ArmstrongClient.zoom();
		if (zoom != null && zoom.active()) {
			cir.setReturnValue(cir.getReturnValueF() / zoom.divisor());
		}
		Mc.fov = cir.getReturnValueF(); // for waypoint markers
	}
	//?} else {
	/*@Inject(method = "getFov", at = @At("RETURN"), cancellable = true)
	private void armstrong$zoom(Camera camera, float partialTick, boolean world, CallbackInfoReturnable<Double> cir) {
		Zoom zoom = ArmstrongClient.zoom();
		if (world && zoom != null && zoom.active()) {
			cir.setReturnValue(cir.getReturnValueD() / zoom.divisor());
		}
		if (world) {
			Mc.fov = (float) cir.getReturnValueD(); // for waypoint markers
		}
	}
	*///?}
}
