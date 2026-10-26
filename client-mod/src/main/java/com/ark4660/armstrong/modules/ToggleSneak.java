package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.compat.Mc;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Indicator;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;

/** Tap sneak once to keep sneaking, tap again to stand up. */
public final class ToggleSneak extends Module implements Indicator {
	private final Setting.Bool showTag = add(new Setting.Bool("show_tag", "show_tag", true));
	private boolean toggled;

	public ToggleSneak() {
		super("toggle_sneak", "Toggle Sneak", "Tap sneak once to keep sneaking.", Category.MOVEMENT, false);
	}

	@Override
	public KeyMapping key() {
		return Minecraft.getInstance().options.keyShift;
	}

	@Override
	public void tick(Minecraft mc) {
		while (mc.options.keyShift.consumeClick()) {
			toggled = !toggled;
			if (!toggled) {
				mc.options.keyShift.setDown(false);
			}
		}
		if (toggled && mc.player != null && Mc.screen() == null) {
			mc.options.keyShift.setDown(true);
		}
	}

	@Override
	protected void onDisable() {
		if (toggled) {
			Minecraft.getInstance().options.keyShift.setDown(false);
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
