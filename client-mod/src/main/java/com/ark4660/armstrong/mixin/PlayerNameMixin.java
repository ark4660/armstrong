package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.cosmetics.CosmeticsModule;

import net.minecraft.client.player.AbstractClientPlayer;
import net.minecraft.network.chat.Component;
import net.minecraft.world.entity.player.Player;

/**
 * Cosmetics: the badge in front of a player's name tag. Only client-side
 * players: in singleplayer the server's players share this class, and their
 * names go into chat and death messages.
 */
@Mixin(Player.class)
public class PlayerNameMixin {
	@Inject(method = "getDisplayName", at = @At("RETURN"), cancellable = true)
	private void armstrong$badge(CallbackInfoReturnable<Component> cir) {
		if ((Object) this instanceof AbstractClientPlayer p) {
			cir.setReturnValue(CosmeticsModule.withBadge(p.getUUID(), cir.getReturnValue()));
		}
	}
}
