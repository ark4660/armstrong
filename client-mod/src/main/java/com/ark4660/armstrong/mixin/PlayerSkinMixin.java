package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.cosmetics.CosmeticsModule;

import net.minecraft.client.player.AbstractClientPlayer;
import net.minecraft.resources.Identifier;

/** Cosmetics: an Armstrong cape replaces the player's cape (and elytra look). */
@Mixin(AbstractClientPlayer.class)
public class PlayerSkinMixin {
	//? if >=26 {
	@Inject(method = "getSkin", at = @At("RETURN"), cancellable = true)
	private void armstrong$cape(CallbackInfoReturnable<net.minecraft.world.entity.player.PlayerSkin> cir) {
		Identifier cape = CosmeticsModule.cape(((AbstractClientPlayer) (Object) this).getUUID());
		if (cape != null) {
			var s = cir.getReturnValue();
			var t = new net.minecraft.core.ClientAsset.ResourceTexture(cape, cape);
			cir.setReturnValue(new net.minecraft.world.entity.player.PlayerSkin(s.body(), t, t, s.model(), s.secure()));
		}
	}
	//?} else {
	/*@Inject(method = "getSkin", at = @At("RETURN"), cancellable = true)
	private void armstrong$cape(CallbackInfoReturnable<net.minecraft.client.resources.PlayerSkin> cir) {
		Identifier cape = CosmeticsModule.cape(((AbstractClientPlayer) (Object) this).getUUID());
		if (cape != null) {
			var s = cir.getReturnValue();
			cir.setReturnValue(new net.minecraft.client.resources.PlayerSkin(s.texture(), s.textureUrl(), cape, cape, s.model(), s.secure()));
		}
	}
	*///?}
}
