package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.cosmetics.CosmeticsModule;

import net.minecraft.client.gui.components.PlayerTabOverlay;
import net.minecraft.client.multiplayer.PlayerInfo;
import net.minecraft.network.chat.Component;

/** Cosmetics: the badge in front of names in the tab list. */
@Mixin(PlayerTabOverlay.class)
public class TabNameMixin {
	@Inject(method = "getNameForDisplay", at = @At("RETURN"), cancellable = true)
	private void armstrong$badge(PlayerInfo info, CallbackInfoReturnable<Component> cir) {
		cir.setReturnValue(CosmeticsModule.withBadge(Mc.profileId(info.getProfile()), cir.getReturnValue()));
	}
}
