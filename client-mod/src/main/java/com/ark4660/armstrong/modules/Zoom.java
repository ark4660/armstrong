package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Indicator;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;

/** Hold the zoom key to zoom in; scroll while holding it to zoom further. */
public final class Zoom extends Module implements Indicator {
	private static final double MIN_FACTOR = 1.5, MAX_FACTOR = 50;

	private final KeyMapping key;
	private final Setting.Number factor = add(new Setting.Number("factor", "factor", 4, 2, 10, 0.5, "x"));
	private final Setting.Bool scroll = add(new Setting.Bool("scroll", "scroll_to_adjust", true));
	private final Setting.Bool smoothCamera = add(new Setting.Bool("smooth_camera", "smooth_camera", false));
	private final Setting.Bool showTag = add(new Setting.Bool("show_tag", "show_tag", true));

	private boolean active;
	private double current;
	private boolean restoreSmooth;

	public Zoom(KeyMapping key) {
		super("zoom", "Zoom", "Hold the zoom key to zoom in.", Category.VIEW, true);
		this.key = key;
	}

	public boolean active() {
		return active;
	}

	@Override
	public KeyMapping key() {
		return key;
	}

	@Override
	public String indicatorLabel() {
		return active && showTag.get() ? "zoom" : null;
	}

	@Override
	public String indicatorValue() {
		return String.format("%.1fx", current);
	}

	/** How much to divide the field of view by while zooming. */
	public float divisor() {
		return (float) current;
	}

	/** Scroll while zoomed: returns true if the scroll was used. */
	public boolean onScroll(double amount) {
		if (!active || !scroll.get() || amount == 0) {
			return false;
		}
		current = Math.clamp(current * (amount > 0 ? 1.25 : 0.8), MIN_FACTOR, MAX_FACTOR);
		return true;
	}

	@Override
	public void tick(Minecraft mc) {
		boolean want = key.isDown() && mc.gui.screen() == null;
		if (want == active) {
			return;
		}
		active = want;
		if (active) {
			current = factor.get();
			if (smoothCamera.get() && !mc.options.smoothCamera) {
				mc.options.smoothCamera = true;
				restoreSmooth = true;
			}
		} else if (restoreSmooth) {
			mc.options.smoothCamera = false;
			restoreSmooth = false;
		}
	}

	@Override
	protected void onDisable() {
		if (restoreSmooth) {
			Minecraft.getInstance().options.smoothCamera = false;
			restoreSmooth = false;
		}
		active = false;
	}
}
