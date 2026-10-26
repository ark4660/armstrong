package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.input.ClickCounter;
import com.ark4660.armstrong.modules.Zoom;
import com.mojang.blaze3d.platform.InputConstants;

import net.minecraft.client.Minecraft;
import net.minecraft.client.MouseHandler;
import net.minecraft.client.input.MouseButtonInfo;

/**
 * While zooming, the scroll wheel changes the zoom instead of the hotbar
 * slot. Also counts clicks for the CPS displays (in-game only, not in menus).
 */
@Mixin(MouseHandler.class)
public class MouseHandlerMixin {
	@Inject(method = "onButton", at = @At("HEAD"))
	private void armstrong$countClick(long window, MouseButtonInfo info, int action, CallbackInfo ci) {
		if (action != InputConstants.PRESS || Minecraft.getInstance().gui.screen() != null) {
			return;
		}
		if (info.button() == InputConstants.MOUSE_BUTTON_LEFT) {
			ClickCounter.click(true);
		} else if (info.button() == InputConstants.MOUSE_BUTTON_RIGHT) {
			ClickCounter.click(false);
		}
	}

	@Inject(method = "onScroll", at = @At("HEAD"), cancellable = true)
	private void armstrong$zoomScroll(long window, double xOffset, double yOffset, CallbackInfo ci) {
		Zoom zoom = ArmstrongClient.zoom();
		if (zoom != null && zoom.onScroll(yOffset)) {
			ci.cancel();
		}
	}
}
