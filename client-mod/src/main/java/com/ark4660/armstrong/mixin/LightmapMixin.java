package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;
import com.llamalad7.mixinextras.injector.ModifyExpressionValue;

/**
 * Fullbright: raises the gamma the lightmap reads. In both versions it's the
 * 2nd option read (after darkness effect scale).
 */
//? if >=26 {
@Mixin(net.minecraft.client.renderer.LightmapRenderStateExtractor.class)
//?} else {
/*@Mixin(net.minecraft.client.renderer.LightTexture.class)
*///?}
public class LightmapMixin {
	//? if >=26 {
	@ModifyExpressionValue(method = "extract", at = @At(value = "INVOKE", target = "Lnet/minecraft/client/OptionInstance;get()Ljava/lang/Object;", ordinal = 1))
	//?} elif >=1.21.11 {
	/*// The third option read is gamma (hideLightningFlash and darknessEffectScale come first).
	@ModifyExpressionValue(method = "updateLightTexture", at = @At(value = "INVOKE", target = "Lnet/minecraft/client/OptionInstance;get()Ljava/lang/Object;", ordinal = 2))
	*///?} else {
	/*@ModifyExpressionValue(method = "updateLightTexture", at = @At(value = "INVOKE", target = "Lnet/minecraft/client/OptionInstance;get()Ljava/lang/Object;", ordinal = 1))
	*///?}
	private Object armstrong$fullbright(Object gamma) {
		ViewModules.Fullbright fb = ArmstrongClient.active(ViewModules.Fullbright.class);
		return fb == null ? gamma : Double.valueOf(fb.gamma());
	}
}
