package com.ark4660.armstrong.module;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

import net.minecraft.client.option.KeyBinding;
import net.minecraft.client.MinecraftClient;

/** A client feature the player can switch on and off in the Armstrong menu. */
public abstract class Module {
	public final String id;
	public final String name;
	public final String description;
	public final Category category;

	private final boolean enabledByDefault;
	private boolean enabled;
	private String blockedBy; // the server that switched this module off, if any
	private final List<Setting<?>> settings = new ArrayList<>();

	protected Module(String id, String name, String description, Category category, boolean enabledByDefault) {
		this.id = id;
		this.name = name;
		this.description = description;
		this.category = category;
		this.enabledByDefault = enabledByDefault;
		this.enabled = enabledByDefault;
	}

	protected <S extends Setting<?>> S add(S setting) {
		settings.add(setting);
		return setting;
	}

	public List<Setting<?>> settings() {
		return Collections.unmodifiableList(settings);
	}

	/** The player's choice (what the menu shows and the config saves). */
	public boolean isEnabled() {
		return enabled;
	}

	/** Whether it's actually running: enabled and not blocked by the server. */
	public boolean isActive() {
		return enabled && blockedBy == null;
	}

	/** The server blocking this module, or null. */
	public String blockedBy() {
		return blockedBy;
	}

	/**
	 * Blocks the module on a server that doesn't allow it (null unblocks).
	 * The player's choice is kept, so it comes back on other servers.
	 */
	public void setBlocked(String server) {
		boolean wasActive = isActive();
		blockedBy = server;
		if (wasActive && !isActive()) {
			onDisable();
		} else if (!wasActive && isActive()) {
			onEnable();
		}
	}

	public void setEnabled(boolean on) {
		if (on == enabled || (!on && !canDisable())) {
			return;
		}
		boolean wasActive = isActive();
		enabled = on;
		if (!wasActive && isActive()) {
			onEnable();
		} else if (wasActive && !isActive()) {
			onDisable();
		}
	}

	public void toggle() {
		setEnabled(!enabled);
	}

	/** Back to defaults: enabled state and every setting. */
	public void reset() {
		setEnabled(enabledByDefault);
		settings.forEach(Setting::reset);
	}

	/** The key this module listens to, shown in the menu; null if none. */
	public KeyBinding key() {
		return null;
	}

	/** Whether the player can switch this module off (the Interface module can't). */
	public boolean canDisable() {
		return true;
	}

	protected void onEnable() {}

	protected void onDisable() {}

	/** Called at the end of every client tick while enabled. */
	public void tick(MinecraftClient mc) {}
}
