package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Indicator;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.client.option.KeyBinding;
import net.minecraft.client.MinecraftClient;

/**
 * Hold the key to look around your player in third person without turning
 * them: the mouse moves the camera (EntityTurnMixin, on increaseTransforms), and the camera uses
 * these angles instead of the player's (GameRendererMixin). Some servers
 * block it (see ServerPolicy).
 */
public final class Freelook extends Module implements Indicator {
	private final KeyBinding key;
	private final Setting.Choice mode = add(new Setting.Choice("mode", "mode", "Hold", "Hold", "Toggle"));
	private final Setting.Bool showTag = add(new Setting.Bool("show_tag", "show_tag", true));

	private boolean active;
	private boolean toggled;
	private float yaw, pitch;
	private Integer before; // options.perspective: 0 first person, 1 back, 2 front

	public Freelook(KeyBinding key) {
		super("freelook", "Freelook", "Hold the key to look around without turning.", Category.VIEW, false);
		this.key = key;
	}

	@Override
	public KeyBinding key() {
		return key;
	}

	/** Whether the camera is detached from the player right now. */
	public boolean active() {
		return active;
	}

	public float yaw() {
		return yaw;
	}

	public float pitch() {
		return pitch;
	}

	/** Mouse movement while active, in the units Entity.increaseTransforms gets (pitch is inverted there). */
	public void turn(double dYaw, double dPitch) {
		yaw += (float) dYaw * 0.15f;
		pitch = Math.clamp(pitch - (float) dPitch * 0.15f, -90f, 90f);
	}

	@Override
	public void tick(MinecraftClient mc) {
		if (mode.get().equals("Toggle")) {
			while (key.wasPressed()) {
				toggled = !toggled;
			}
		} else {
			toggled = false;
		}
		boolean want = (toggled || Mc.held(key)) && mc.player != null && Mc.screen() == null;
		if (want == active) {
			return;
		}
		active = want;
		if (active) {
			yaw = mc.player.yaw;
			pitch = mc.player.pitch;
			// Always the rear view: the camera's angles are replaced, so a front
			// view would put the camera on the wrong side.
			before = mc.options.perspective;
			mc.options.perspective = 1;
		} else {
			restore(mc);
		}
	}

	private void restore(MinecraftClient mc) {
		if (before != null) {
			mc.options.perspective = before;
			before = null;
		}
	}

	@Override
	protected void onDisable() {
		if (active) {
			restore(MinecraftClient.getInstance());
		}
		active = false;
		toggled = false;
	}

	@Override
	public String indicatorLabel() {
		return active && showTag.get() ? "freelook" : null;
	}

	@Override
	public String indicatorValue() {
		return "on";
	}
}
