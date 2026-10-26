package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.ModifyVariable;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.Freelook;

import net.minecraft.client.Camera;

/** Freelook: the camera takes Freelook's angles instead of the player's. */
@Mixin(Camera.class)
public class FreelookCameraMixin {
	@ModifyVariable(method = "setRotation", at = @At("HEAD"), argsOnly = true, ordinal = 0)
	private float armstrong$freelookYaw(float yRot) {
		Freelook f = ArmstrongClient.active(Freelook.class);
		return f != null && f.active() ? f.yaw() : yRot;
	}

	@ModifyVariable(method = "setRotation", at = @At("HEAD"), argsOnly = true, ordinal = 1)
	private float armstrong$freelookPitch(float xRot) {
		Freelook f = ArmstrongClient.active(Freelook.class);
		return f != null && f.active() ? f.pitch() : xRot;
	}
}
