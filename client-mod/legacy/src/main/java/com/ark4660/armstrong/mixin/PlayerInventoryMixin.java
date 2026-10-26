package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.Zoom;

import net.minecraft.entity.player.PlayerInventory;

/** While zooming, the scroll wheel changes the zoom instead of the hotbar slot. */
@Mixin(PlayerInventory.class)
public class PlayerInventoryMixin {
	@Inject(method = "scrollInHotbar", at = @At("HEAD"), cancellable = true)
	private void armstrong$zoomScroll(int amount, CallbackInfo ci) {
		Zoom zoom = ArmstrongClient.zoom();
		if (zoom != null && zoom.onScroll(amount)) {
			ci.cancel();
		}
	}
}
