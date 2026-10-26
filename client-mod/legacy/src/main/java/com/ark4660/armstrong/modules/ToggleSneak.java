package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.compat.Mc;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Indicator;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.option.KeyBinding;

/** Tap sneak once to keep sneaking, tap again to stand up. */
public final class ToggleSneak extends Module implements Indicator {
	private final Setting.Bool showTag = add(new Setting.Bool("show_tag", "show_tag", true));
	private boolean toggled;

	public ToggleSneak() {
		super("toggle_sneak", "Toggle Sneak", "Tap sneak once to keep sneaking.", Category.MOVEMENT, false);
	}

	@Override
	public KeyBinding key() {
		return MinecraftClient.getInstance().options.sneakKey;
	}

	@Override
	public void tick(MinecraftClient mc) {
		while (mc.options.sneakKey.wasPressed()) {
			toggled = !toggled;
			if (!toggled) {
				KeyBinding.setKeyPressed(mc.options.sneakKey.getCode(), false);
			}
		}
		if (toggled && mc.player != null && Mc.screen() == null) {
			KeyBinding.setKeyPressed(mc.options.sneakKey.getCode(), true);
		}
	}

	@Override
	protected void onDisable() {
		if (toggled) {
			KeyBinding sneak = MinecraftClient.getInstance().options.sneakKey;
			KeyBinding.setKeyPressed(sneak.getCode(), false);
		}
		toggled = false;
	}

	@Override
	public String indicatorLabel() {
		return isActive() && toggled && showTag.get() ? "sneak" : null;
	}

	@Override
	public String indicatorValue() {
		return "toggled";
	}
}
