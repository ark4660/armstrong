package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.cosmetics.CosmeticsModule;

import net.minecraft.client.network.AbstractClientPlayerEntity;
import net.minecraft.entity.player.PlayerEntity;
import net.minecraft.text.Text;

/**
 * Cosmetics: the badge in front of a player's name tag (1.8.9's getName is
 * the display name). Only client-side players: in singleplayer the server's
 * players share this class, and their names go into chat.
 */
@Mixin(PlayerEntity.class)
public class PlayerNameMixin {
	@Inject(method = "getName", at = @At("RETURN"), cancellable = true)
	private void armstrong$badge(CallbackInfoReturnable<Text> cir) {
		if ((Object) this instanceof AbstractClientPlayerEntity p) {
			cir.setReturnValue(CosmeticsModule.withBadge(p.getUuid(), cir.getReturnValue()));
		}
	}
}
