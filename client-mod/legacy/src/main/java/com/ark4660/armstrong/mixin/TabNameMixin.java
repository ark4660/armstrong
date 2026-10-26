package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.cosmetics.CosmeticsModule;

import net.minecraft.client.gui.hud.PlayerListHud;
import net.minecraft.client.network.PlayerListEntry;

/** Cosmetics: the badge in front of names in the tab list. */
@Mixin(PlayerListHud.class)
public class TabNameMixin {
	@Inject(method = "getPlayerName", at = @At("RETURN"), cancellable = true)
	private void armstrong$badge(PlayerListEntry entry, CallbackInfoReturnable<String> cir) {
		cir.setReturnValue(CosmeticsModule.withBadge(entry.getProfile().getId(), cir.getReturnValue()));
	}
}
