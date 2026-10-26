package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.cosmetics.CosmeticsModule;

import net.minecraft.client.network.AbstractClientPlayerEntity;
import net.minecraft.util.Identifier;

/**
 * Cosmetics: an Armstrong cape replaces the player's cape. (In 1.8.9's names
 * the cape texture is getSkinId; the cape renderer binds what it returns.)
 */
@Mixin(AbstractClientPlayerEntity.class)
public class PlayerCapeMixin {
	@Inject(method = "getSkinId", at = @At("RETURN"), cancellable = true)
	private void armstrong$cape(CallbackInfoReturnable<Identifier> cir) {
		Identifier cape = CosmeticsModule.cape(((AbstractClientPlayerEntity) (Object) this).getUuid());
		if (cape != null) {
			cir.setReturnValue(cape);
		}
	}
}
