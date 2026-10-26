package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.modules.Freelook;
import com.ark4660.armstrong.modules.ViewModules;
import com.ark4660.armstrong.modules.Zoom;
import com.llamalad7.mixinextras.injector.ModifyExpressionValue;

import net.minecraft.client.render.GameRenderer;

/** Zoom narrows the world FOV; Fullbright raises the gamma the lightmap reads; Freelook moves the camera. */
@Mixin(GameRenderer.class)
public class GameRendererMixin {
	// changingFov is false for the hand, which keeps its own FOV.
	@Inject(method = "getFov", at = @At("RETURN"), cancellable = true)
	private void armstrong$zoom(float tickDelta, boolean changingFov, CallbackInfoReturnable<Float> cir) {
		Zoom zoom = ArmstrongClient.zoom();
		if (changingFov && zoom != null && zoom.active()) {
			cir.setReturnValue(cir.getReturnValueF() / zoom.divisor());
		}
		if (changingFov) {
			Mc.fov = cir.getReturnValueF(); // for waypoint markers
		}
	}

	// Freelook: while the camera is placed, the view entity's angles read as
	// Freelook's. Current and previous both, so there's no interpolation.
	@ModifyExpressionValue(method = "transformCamera", at = {
			@At(value = "FIELD", target = "Lnet/minecraft/entity/Entity;yaw:F"),
			@At(value = "FIELD", target = "Lnet/minecraft/entity/Entity;prevYaw:F")})
	private float armstrong$freelookYaw(float yaw) {
		Freelook f = ArmstrongClient.active(Freelook.class);
		return f != null && f.active() ? f.yaw() : yaw;
	}

	@ModifyExpressionValue(method = "transformCamera", at = {
			@At(value = "FIELD", target = "Lnet/minecraft/entity/Entity;pitch:F"),
			@At(value = "FIELD", target = "Lnet/minecraft/entity/Entity;prevPitch:F")})
	private float armstrong$freelookPitch(float pitch) {
		Freelook f = ArmstrongClient.active(Freelook.class);
		return f != null && f.active() ? f.pitch() : pitch;
	}

	@ModifyExpressionValue(method = "updateLightmap", at = @At(value = "FIELD", target = "Lnet/minecraft/client/option/GameOptions;gamma:F"))
	private float armstrong$fullbright(float gamma) {
		ViewModules.Fullbright fb = ArmstrongClient.active(ViewModules.Fullbright.class);
		return fb == null ? gamma : fb.gamma();
	}
}
