package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Indicator;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.client.CameraType;
import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;

/**
 * Hold the key to look around your player in third person without turning
 * them: the mouse moves the camera (EntityTurnMixin), and the camera uses
 * these angles instead of the player's (FreelookCameraMixin). Some servers
 * block it (see ServerPolicy).
 */
public final class Freelook extends Module implements Indicator {
	private final KeyMapping key;
	private final Setting.Choice mode = add(new Setting.Choice("mode", "mode", "Hold", "Hold", "Toggle"));
	private final Setting.Bool showTag = add(new Setting.Bool("show_tag", "show_tag", true));

	private boolean active;
	private boolean toggled;
	private float yaw, pitch;
	private CameraType before;

	public Freelook(KeyMapping key) {
		super("freelook", "Freelook", "Hold the key to look around without turning.", Category.VIEW, false);
		this.key = key;
	}

	@Override
	public KeyMapping key() {
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

	/** Mouse movement while active, in the units Entity.turn gets. */
	public void turn(double dYaw, double dPitch) {
		yaw += (float) dYaw * 0.15f;
		pitch = Math.clamp(pitch + (float) dPitch * 0.15f, -90f, 90f);
	}

	@Override
	public void tick(Minecraft mc) {
		if (mode.get().equals("Toggle")) {
			while (key.consumeClick()) {
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
			yaw = mc.player.getYRot();
			pitch = mc.player.getXRot();
			// Always the rear view: the camera's angles are replaced, so a front
			// view would put the camera on the wrong side.
			before = mc.options.getCameraType();
			mc.options.setCameraType(CameraType.THIRD_PERSON_BACK);
		} else {
			restore(mc);
		}
	}

	private void restore(Minecraft mc) {
		if (before != null) {
			mc.options.setCameraType(before);
			before = null;
		}
	}

	@Override
	protected void onDisable() {
		if (active) {
			restore(Minecraft.getInstance());
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
