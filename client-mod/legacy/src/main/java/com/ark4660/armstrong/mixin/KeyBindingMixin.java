package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.input.ClickCounter;

import net.minecraft.client.option.KeyBinding;

/**
 * Counts clicks for the CPS displays. The game reports every in-game key and
 * mouse press here; mouse buttons are codes -100 (left) and -99 (right).
 */
@Mixin(KeyBinding.class)
public class KeyBindingMixin {
	@Inject(method = "onKeyPressed", at = @At("HEAD"))
	private static void armstrong$countClick(int code, CallbackInfo ci) {
		if (Mc.screen() != null) {
			return;
		}
		if (code == -100) {
			ClickCounter.click(true);
		} else if (code == -99) {
			ClickCounter.click(false);
		}
	}
}
