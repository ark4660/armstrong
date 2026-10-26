package com.ark4660.armstrong.home;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.Interface;

import net.minecraft.client.gui.screen.Screen;
import net.minecraft.client.gui.screen.TitleScreen;

/** Picks the home screen for the Interface setting. */
public final class HomeScreens {
	private HomeScreens() {}

	public static HomeScreen create(String style) {
		return Interface.HOME_PROMPT.equals(style) ? new PromptHomeScreen() : new BootHomeScreen();
	}

	/**
	 * Swaps the vanilla title screen for Armstrong's, unless the player chose
	 * the vanilla one. Only the plain title screen is replaced.
	 */
	public static Screen replace(Screen screen) {
		if (screen == null || screen.getClass() != TitleScreen.class || ArmstrongClient.ui() == null) {
			return screen;
		}
		String style = ArmstrongClient.ui().homeScreen();
		return Interface.HOME_VANILLA.equals(style) ? screen : create(style);
	}
}
