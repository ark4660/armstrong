package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;

import net.minecraft.client.render.entity.EntityRenderer;
import net.minecraft.entity.Entity;
import net.minecraft.entity.player.PlayerEntity;

/** Hide Name Tags: every label above an entity (names, scores below them) is drawn here in 1.8.9. */
@Mixin(EntityRenderer.class)
public class NameTagMixin {
	@Inject(method = "renderLabelIfPresent", at = @At("HEAD"), cancellable = true)
	private void armstrong$hideNameTag(Entity entity, String text, double x, double y, double z, int range, CallbackInfo ci) {
		ViewModules.HideNameTags hide = ArmstrongClient.active(ViewModules.HideNameTags.class);
		if (hide != null && hide.hides(entity instanceof PlayerEntity)) {
			ci.cancel();
		}
	}
}
