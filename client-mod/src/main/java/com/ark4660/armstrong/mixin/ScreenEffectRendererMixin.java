package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;
import com.mojang.blaze3d.vertex.PoseStack;

import net.minecraft.client.renderer.ScreenEffectRenderer;

/** Low Fire: shifts the burning overlay down while it's drawn. */
@Mixin(ScreenEffectRenderer.class)
public class ScreenEffectRendererMixin {
	//? if >=26 {
	@Inject(method = "submitFire", at = @At("HEAD"))
	private static void armstrong$lowerFire(PoseStack pose, net.minecraft.client.renderer.SubmitNodeCollector collector, net.minecraft.client.renderer.texture.TextureAtlasSprite sprite, CallbackInfo ci) {
		lower(pose);
	}

	@Inject(method = "submitFire", at = @At("RETURN"))
	private static void armstrong$restore(PoseStack pose, net.minecraft.client.renderer.SubmitNodeCollector collector, net.minecraft.client.renderer.texture.TextureAtlasSprite sprite, CallbackInfo ci) {
		pose.popPose();
	}
	//?} else {
	/*@Inject(method = "renderFire", at = @At("HEAD"))
	private static void armstrong$lowerFire(net.minecraft.client.Minecraft mc, PoseStack pose, CallbackInfo ci) {
		lower(pose);
	}

	@Inject(method = "renderFire", at = @At("RETURN"))
	private static void armstrong$restore(net.minecraft.client.Minecraft mc, PoseStack pose, CallbackInfo ci) {
		pose.popPose();
	}
	*///?}

	private static void lower(PoseStack pose) {
		pose.pushPose();
		ViewModules.LowFire lf = ArmstrongClient.active(ViewModules.LowFire.class);
		if (lf != null) {
			pose.translate(0f, -lf.offset(), 0f);
		}
	}
}
