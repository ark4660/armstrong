package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.InfoHuds;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.network.ClientPlayerInteractionManager;
import net.minecraft.entity.Entity;
import net.minecraft.entity.player.PlayerEntity;

/** Hits you make, for PvP Info's combo and reach (Legacy Fabric has no attack event). */
@Mixin(ClientPlayerInteractionManager.class)
public class InteractionManagerMixin {
	@Inject(method = "attackEntity", at = @At("HEAD"))
	private void armstrong$attack(PlayerEntity player, Entity target, CallbackInfo ci) {
		InfoHuds.PvpInfo pvp = ArmstrongClient.active(InfoHuds.PvpInfo.class);
		if (pvp != null) {
			pvp.onAttack(MinecraftClient.getInstance(), target);
		}
	}
}
