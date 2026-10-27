package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;
import com.llamalad7.mixinextras.injector.ModifyExpressionValue;
import com.llamalad7.mixinextras.sugar.Local;

import net.minecraft.client.renderer.entity.EntityRenderer;
import net.minecraft.world.entity.Entity;
import net.minecraft.world.entity.player.Player;

/**
 * Hide Name Tags: answers "no" where the renderer asks whether to draw an
 * entity's name. The call site, not shouldShowName itself, because renderers
 * override that without calling the base method.
 */
@Mixin(EntityRenderer.class)
public class NameTagMixin {
	//? if >=26 {
	// Two extractNameTags exist; the one taking camera distances asks shouldShowName.
	@ModifyExpressionValue(method = "extractNameTags(Lnet/minecraft/world/entity/Entity;Lnet/minecraft/client/renderer/entity/state/EntityRenderState;FDD)V", at = @At(value = "INVOKE",
			target = "Lnet/minecraft/client/renderer/entity/EntityRenderer;shouldShowName(Lnet/minecraft/world/entity/Entity;D)Z"))
	//?} elif >=1.21.11 {
	/*@ModifyExpressionValue(method = "extractRenderState", at = @At(value = "INVOKE",
			target = "Lnet/minecraft/client/renderer/entity/EntityRenderer;shouldShowName(Lnet/minecraft/world/entity/Entity;D)Z"))
	*///?} else {
	/*@ModifyExpressionValue(method = "render", at = @At(value = "INVOKE",
			target = "Lnet/minecraft/client/renderer/entity/EntityRenderer;shouldShowName(Lnet/minecraft/world/entity/Entity;)Z"))
	*///?}
	private boolean armstrong$hideNameTag(boolean show, @Local(argsOnly = true) Entity entity) {
		ViewModules.HideNameTags hide = ArmstrongClient.active(ViewModules.HideNameTags.class);
		return show && (hide == null || !hide.hides(entity instanceof Player));
	}
}
