package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

/** The client's own look: accent color and the terminal font. Always on. */
public final class Interface extends Module {
	private final Setting.Choice accent = add(new Setting.Choice("accent", "accent", "Green", "Green", "Amber", "Cyan"));
	private final Setting.Bool monospace = add(new Setting.Bool("monospace", "monospace", true));
	private final Setting.Choice homeScreen = add(new Setting.Choice("home_screen", "home_screen", HOME_BOOT,
			HOME_BOOT, HOME_PROMPT, HOME_VANILLA));

	public static final String HOME_BOOT = "Boot terminal";
	public static final String HOME_PROMPT = "Minimal prompt";
	public static final String HOME_VANILLA = "Vanilla";

	public Interface() {
		super("interface", "Interface", "The client's look: accent color, terminal font and home screen.", Category.CLIENT, true);
	}

	public String accent() {
		return accent.get();
	}

	public boolean monospace() {
		return monospace.get();
	}

	public String homeScreen() {
		return homeScreen.get();
	}

	public void setHomeScreen(String style) {
		homeScreen.set(style);
	}

	@Override
	public boolean canDisable() {
		return false;
	}
}
