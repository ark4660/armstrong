package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;
import com.llamalad7.mixinextras.injector.ModifyExpressionValue;

import net.minecraft.client.renderer.LightmapRenderStateExtractor;

/** Fullbright: raises the gamma the lightmap reads (the 2nd option read in extract). */
@Mixin(LightmapRenderStateExtractor.class)
public class LightmapMixin {
	@ModifyExpressionValue(method = "extract", at = @At(value = "INVOKE", target = "Lnet/minecraft/client/OptionInstance;get()Ljava/lang/Object;", ordinal = 1))
	private Object armstrong$fullbright(Object gamma) {
		ViewModules.Fullbright fb = ArmstrongClient.active(ViewModules.Fullbright.class);
		return fb == null ? gamma : Double.valueOf(fb.gamma());
	}
}
