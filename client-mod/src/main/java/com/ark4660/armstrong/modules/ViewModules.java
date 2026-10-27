package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.compat.Mc;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Indicator;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.client.CameraType;
import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;
//? if >=1.21.11 {
import net.minecraft.client.gui.components.debug.DebugScreenEntries;
import net.minecraft.client.gui.components.debug.DebugScreenEntryStatus;
//?}

/** Small view modules. The heavy lifting is in the mixins that read them. */
public final class ViewModules {
	private ViewModules() {}

	/** Full brightness everywhere, by raising gamma for the lightmap only. */
	public static final class Fullbright extends Module {
		private final Setting.Number level = add(new Setting.Number("level", "brightness", 100, 10, 100, 5, "%"));

		public Fullbright() {
			super("fullbright", "Fullbright", "See in the dark: maximum brightness.", Category.VIEW, false);
		}

		/** The gamma the lightmap uses; vanilla's slider tops out at 1.0. */
		public double gamma() {
			return 1.0 + level.get() / 100.0 * 15.0;
		}
	}

	/** Client-side time of day: your sky only, the server is unaffected. */
	public static final class TimeChanger extends Module {
		private final Setting.Choice time = add(new Setting.Choice("time", "time", "Noon",
				"Sunrise", "Noon", "Sunset", "Midnight"));

		public TimeChanger() {
			super("time_changer", "Time Changer", "Pick the time of day you see (only on your screen).", Category.VIEW, false);
		}

		/** Ticks into the day cycle (24000 per day). */
		public long ticks() {
			return switch (time.get()) {
				case "Sunrise" -> 23000;
				case "Sunset" -> 12500;
				case "Midnight" -> 18000;
				default -> 6000;
			};
		}
	}

	/** Client-side weather. */
	public static final class WeatherChanger extends Module {
		private final Setting.Choice weather = add(new Setting.Choice("weather", "weather", "Clear", "Clear", "Rain", "Thunder"));

		public WeatherChanger() {
			super("weather_changer", "Weather Changer", "Pick the weather you see (only on your screen).", Category.VIEW, false);
		}

		public float rain() {
			return weather.get().equals("Clear") ? 0f : 1f;
		}

		public float thunder() {
			return weather.get().equals("Thunder") ? 1f : 0f;
		}
	}

	/** Moves the burning overlay down so it covers less of the screen. */
	/** Hides the names above players' heads, and optionally named mobs' and armor stands'. */
	public static final class HideNameTags extends Module {
		private final Setting.Bool players = add(new Setting.Bool("players", "players", true));
		private final Setting.Bool mobs = add(new Setting.Bool("mobs", "mobs", false));

		public HideNameTags() {
			super("hide_name_tags", "Hide Name Tags", "Hide the names above other players (and named mobs, if you like).", Category.VIEW, false);
		}

		/** Whether to hide the name tag of a player (true) or of anything else. */
		public boolean hides(boolean player) {
			return player ? players.get() : mobs.get();
		}
	}

	public static final class LowFire extends Module {
		private final Setting.Number lower = add(new Setting.Number("lower", "lower_by", 30, 0, 50, 5, "%"));

		public LowFire() {
			super("low_fire", "Low Fire", "The burning overlay covers less of the screen.", Category.VIEW, true);
		}

		public float offset() {
			return (float) (lower.get() / 100.0);
		}
	}

	/** Fewer particles: for clarity in fights and for performance. */
	public static final class Particles extends Module {
		private final Setting.Number amount = add(new Setting.Number("amount", "amount", 50, 0, 100, 5, "%"));

		public Particles() {
			super("particles", "Particles", "Show fewer particles.", Category.VIEW, false);
		}

		public double keep() {
			return amount.get() / 100.0;
		}
	}

	/** Switches one of the game's debug renderers (F3 toggles) with the module. */
	public abstract static class DebugToggle extends Module {
		protected DebugToggle(String id, String name, String description) {
			super(id, name, description, Category.VIEW, false);
		}

		/** Turns the game's renderer on or off. */
		protected abstract void set(Minecraft mc, boolean on);

		private boolean applied;

		// Applied on the first tick, not at enable time: a module loaded as
		// "on" is enabled while the game is still starting up.
		@Override
		public void tick(Minecraft mc) {
			if (!applied) {
				set(mc, true);
				applied = true;
			}
		}

		@Override
		protected void onDisable() {
			if (applied) {
				set(Minecraft.getInstance(), false);
				applied = false;
			}
		}
	}

	public static final class Hitboxes extends DebugToggle {
		public Hitboxes() {
			super("hitboxes", "Hitboxes", "Show entity hitboxes (like F3+B).");
		}

		@Override
		protected void set(Minecraft mc, boolean on) {
			//? if >=1.21.11 {
			mc.debugEntries.setStatus(DebugScreenEntries.ENTITY_HITBOXES, on ? DebugScreenEntryStatus.ALWAYS_ON : DebugScreenEntryStatus.NEVER);
			//?} else {
			/*mc.getEntityRenderDispatcher().setRenderHitBoxes(on);
			*///?}
		}
	}

	public static final class ChunkBorders extends DebugToggle {
		public ChunkBorders() {
			super("chunk_borders", "Chunk Borders", "Show chunk borders (like F3+G).");
		}

		@Override
		protected void set(Minecraft mc, boolean on) {
			//? if >=1.21.11 {
			mc.debugEntries.setStatus(DebugScreenEntries.CHUNK_BORDERS, on ? DebugScreenEntryStatus.ALWAYS_ON : DebugScreenEntryStatus.NEVER);
			//?} else {
			/*if (mc.debugRenderer.switchRenderChunkborder() != on) {
				mc.debugRenderer.switchRenderChunkborder(); // it only toggles
			}
			*///?}
		}
	}

	/** Hold a key to look at yourself from the front, release to go back. */
	public static final class Snaplook extends Module implements Indicator {
		private final KeyMapping key;
		private CameraType before;

		public Snaplook(KeyMapping key) {
			super("snaplook", "Snaplook", "Hold the key to see yourself from the front.", Category.VIEW, true);
			this.key = key;
		}

		@Override
		public KeyMapping key() {
			return key;
		}

		@Override
		public void tick(Minecraft mc) {
			boolean held = Mc.held(key) && Mc.screen() == null;
			if (held && before == null) {
				before = mc.options.getCameraType();
				mc.options.setCameraType(CameraType.THIRD_PERSON_FRONT);
			} else if (!held && before != null) {
				mc.options.setCameraType(before);
				before = null;
			}
		}

		@Override
		protected void onDisable() {
			if (before != null) {
				Minecraft.getInstance().options.setCameraType(before);
				before = null;
			}
		}

		@Override
		public String indicatorLabel() {
			return before != null ? "snaplook" : null;
		}

		@Override
		public String indicatorValue() {
			return "on";
		}
	}
}
