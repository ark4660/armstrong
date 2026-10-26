package com.ark4660.armstrong.compat;

import java.util.Locale;

import org.lwjgl.input.Keyboard;
import org.lwjgl.input.Mouse;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.gui.screen.Screen;
import net.minecraft.client.option.GameOptions;
import net.minecraft.client.option.KeyBinding;

/** The 1.8.9 versions of the modern builds' compat.Mc calls. */
public final class Mc {
	private Mc() {}

	/** The open screen, or null in-game. */
	public static Screen screen() {
		return MinecraftClient.getInstance().currentScreen;
	}

	/** Opens a screen (null closes it). */
	public static void open(Screen screen) {
		MinecraftClient.getInstance().setScreen(screen);
	}

	public static String versionName() {
		return "1.8.9";
	}

	/** Whether a key press in a screen is the key bound to a binding. */
	public static boolean isBound(KeyBinding binding, int key) {
		return binding.getCode() > 0 && binding.getCode() == key;
	}

	/**
	 * Whether a binding's key is held. Like 1.21.1, only one binding per key
	 * registers as held (zoom on C would lose to anything else on C), so the
	 * physical key is read too. Mouse buttons are negative codes (-100 = left).
	 */
	public static boolean held(KeyBinding binding) {
		if (binding.isPressed()) {
			return true;
		}
		int code = binding.getCode();
		if (code < 0) {
			return Mouse.isCreated() && Mouse.isButtonDown(code + 100);
		}
		return code > 0 && Keyboard.isCreated() && Keyboard.isKeyDown(code);
	}

	/** The key's name for the menu, like right_shift; "none" when unbound. */
	public static String keyName(KeyBinding binding) {
		if (binding == null) {
			return "-";
		}
		if (binding.getCode() == 0) {
			return "none";
		}
		return GameOptions.getFormattedNameForKeyCode(binding.getCode()).toLowerCase(Locale.ROOT).replace(' ', '_');
	}
}
