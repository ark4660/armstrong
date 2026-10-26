package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.compat.Mc;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Indicator;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;

/**
 * Tap the sprint key once to keep sprinting, tap again to stop. It holds the
 * vanilla sprint key down while toggled, so the game's own rules (hunger,
 * blindness, sneaking) still decide whether you actually sprint.
 */
public final class ToggleSprint extends Module implements Indicator {
	private final Setting.Bool showTag = add(new Setting.Bool("show_tag", "show_tag", true));
	private boolean toggled;

	public ToggleSprint() {
		super("toggle_sprint", "Toggle Sprint", "Tap sprint once to keep sprinting.", Category.MOVEMENT, false);
	}

	public boolean isToggled() {
		return isEnabled() && toggled;
	}

	@Override
	public KeyMapping key() {
		return Minecraft.getInstance().options.keySprint;
	}

	@Override
	public String indicatorLabel() {
		return isToggled() && showTag.get() ? "sprint" : null;
	}

	@Override
	public String indicatorValue() {
		return "toggled";
	}

	@Override
	public void tick(Minecraft mc) {
		while (mc.options.keySprint.consumeClick()) {
			toggled = !toggled;
		}
		if (toggled && mc.player != null && Mc.screen() == null) {
			mc.options.keySprint.setDown(true);
		}
	}

	@Override
	protected void onDisable() {
		if (toggled) {
			Minecraft.getInstance().options.keySprint.setDown(false);
		}
		toggled = false;
	}
}
