package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.ModifyVariable;

import com.ark4660.armstrong.home.HomeScreens;

import net.minecraft.client.gui.Gui;
import net.minecraft.client.gui.screens.Screen;

/**
 * Replaces the title screen with Armstrong's home screen. A TitleScreen can
 * arrive as the argument, or be built inside the method (setScreen(null)
 * with no world assigns `screen = new TitleScreen()`), so both the argument
 * and every store into it are filtered.
 */
@Mixin(Gui.class)
public class GuiMixin {
	@ModifyVariable(method = "setScreen", argsOnly = true, at = @At("HEAD"))
	private Screen armstrong$homeScreenArg(Screen screen) {
		return HomeScreens.replace(screen);
	}

	@ModifyVariable(method = "setScreen", argsOnly = true, at = @At("STORE"))
	private Screen armstrong$homeScreenStored(Screen screen) {
		return HomeScreens.replace(screen);
	}
}
