package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.Freelook;

import net.minecraft.client.MinecraftClient;
import net.minecraft.entity.Entity;

/** Freelook: mouse movement turns the camera, not your player. */
@Mixin(Entity.class)
public class EntityTurnMixin {
	@Inject(method = "increaseTransforms", at = @At("HEAD"), cancellable = true)
	private void armstrong$freelook(float yaw, float pitch, CallbackInfo ci) {
		if ((Object) this != MinecraftClient.getInstance().player) {
			return;
		}
		Freelook f = ArmstrongClient.active(Freelook.class);
		if (f != null && f.active()) {
			f.turn(yaw, pitch);
			ci.cancel();
		}
	}
}
