package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.compat.Mc;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Indicator;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.option.KeyBinding;

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
	public KeyBinding key() {
		return MinecraftClient.getInstance().options.sprintKey;
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
	public void tick(MinecraftClient mc) {
		while (mc.options.sprintKey.wasPressed()) {
			toggled = !toggled;
		}
		if (toggled && mc.player != null && Mc.screen() == null) {
			KeyBinding.setKeyPressed(mc.options.sprintKey.getCode(), true);
		}
	}

	@Override
	protected void onDisable() {
		if (toggled) {
			KeyBinding sprint = MinecraftClient.getInstance().options.sprintKey;
			KeyBinding.setKeyPressed(sprint.getCode(), false);
		}
		toggled = false;
	}
}
