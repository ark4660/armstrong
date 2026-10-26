package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.Freelook;

import net.minecraft.client.Minecraft;
import net.minecraft.world.entity.Entity;

/** Freelook: mouse movement turns the camera, not your player. */
@Mixin(Entity.class)
public class EntityTurnMixin {
	@Inject(method = "turn", at = @At("HEAD"), cancellable = true)
	private void armstrong$freelook(double dYaw, double dPitch, CallbackInfo ci) {
		if ((Object) this != Minecraft.getInstance().player) {
			return;
		}
		Freelook f = ArmstrongClient.active(Freelook.class);
		if (f != null && f.active()) {
			f.turn(dYaw, dPitch);
			ci.cancel();
		}
	}
}
