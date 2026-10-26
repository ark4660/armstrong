package com.ark4660.armstrong.modules;

import java.util.ArrayList;
import java.util.List;

import com.ark4660.armstrong.input.ClickCounter;
import com.ark4660.armstrong.module.HudModule;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import java.time.LocalTime;
import java.time.format.DateTimeFormatter;

import net.fabricmc.loader.api.FabricLoader;
import net.minecraft.client.MinecraftClient;
import net.minecraft.client.network.PlayerListEntry;
import net.minecraft.util.math.BlockPos;

/** The status block: fps, ping, coordinates, facing and clicks per second. */
public final class StatusHud extends HudModule.Text {
	private final Setting.Bool header = add(new Setting.Bool("header", "header", true));
	private final Setting.Bool fps = add(new Setting.Bool("fps", "fps", true));
	private final Setting.Bool ping = add(new Setting.Bool("ping", "ping", true));
	private final Setting.Bool coords = add(new Setting.Bool("coords", "coords", true));
	private final Setting.Bool facing = add(new Setting.Bool("facing", "facing", true));
	private final Setting.Bool cps = add(new Setting.Bool("cps", "cps", true));
	private final Setting.Bool clock = add(new Setting.Bool("clock", "clock", false));
	private final Setting.Bool playtime = add(new Setting.Bool("playtime", "playtime", false));
	private final Setting.Bool speed = add(new Setting.Bool("speed", "speed", false));
	private final Setting.Bool saturation = add(new Setting.Bool("saturation", "saturation", false));
	private final Setting.Bool biome = add(new Setting.Bool("biome", "biome", false));
	private final Setting.Bool memory = add(new Setting.Bool("memory", "memory", false));
	private final Setting.Bool server = add(new Setting.Bool("server", "server", false));

	private long joinedAt = System.currentTimeMillis();
	private boolean hasLast;
	private double lastX, lastZ;
	private double blocksPerSecond;

	private final String version = FabricLoader.getInstance().getModContainer("armstrong")
			.map(c -> c.getMetadata().getVersion().getFriendlyString()).orElse("");

	public StatusHud() {
		super("status", "Status", "fps, ping, coordinates, facing and clicks per second.", true, "Top left");
	}

	@Override
	protected List<Term.Line> lines(MinecraftClient mc) {
		int accent = Theme.accent();
		List<Term.Line> out = new ArrayList<>();
		if (header.get()) {
			out.add(new Term.Line().add("armstrong " + version + " ", accent).add("─".repeat(8), Theme.MUTED));
		}
		if (fps.get()) {
			out.add(row("fps", String.valueOf(MinecraftClient.getCurrentFps()), accent));
		}
		if (ping.get()) {
			out.add(row("ping", latency(mc), accent));
		}
		if (mc.player != null) {
			if (coords.get()) {
				out.add(row("xyz", String.format("%d %d %d",
						(int) Math.floor(mc.player.x), (int) Math.floor(mc.player.getBoundingBox().minY), (int) Math.floor(mc.player.z)), Theme.TEXT));
			}
			if (facing.get()) {
				out.add(row("dir", compass(mc.player.yaw), Theme.TEXT));
			}
		}
		if (cps.get()) {
			out.add(new Term.Line().add(Term.pad("cps", 7), Theme.MUTED)
					.add(String.valueOf(ClickCounter.left()), accent).add(" | ", Theme.MUTED)
					.add(String.valueOf(ClickCounter.right()), accent));
		}
		if (clock.get()) {
			out.add(row("time", LocalTime.now().format(CLOCK), Theme.TEXT));
		}
		if (playtime.get()) {
			long s = (System.currentTimeMillis() - joinedAt) / 1000;
			out.add(row("played", String.format("%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60), Theme.TEXT));
		}
		if (mc.player != null) {
			if (speed.get()) {
				out.add(row("speed", String.format("%.1f b/s", blocksPerSecond), Theme.TEXT));
			}
			if (saturation.get()) {
				out.add(row("food", mc.player.getHungerManager().getFoodLevel() + " · sat " + String.format("%.1f", mc.player.getHungerManager().getSaturationLevel()), Theme.TEXT));
			}
			if (biome.get() && mc.world != null) {
				String name = mc.world.getBiome(new BlockPos(mc.player)).name.toLowerCase(java.util.Locale.ROOT).replace(' ', '_');
				out.add(row("biome", name, Theme.TEXT));
			}
		}
		if (memory.get()) {
			Runtime rt = Runtime.getRuntime();
			long used = (rt.totalMemory() - rt.freeMemory()) >> 20, max = rt.maxMemory() >> 20;
			out.add(row("mem", used + "/" + max + " MB", used * 10 > max * 9 ? Theme.AMBER : Theme.TEXT));
		}
		if (server.get()) {
			var sd = mc.getCurrentServerEntry();
			out.add(row("server", sd != null ? sd.address : mc.isInSingleplayer() ? "singleplayer" : "-", Theme.TEXT));
		}
		return out;
	}

	private static final DateTimeFormatter CLOCK = DateTimeFormatter.ofPattern("HH:mm");

	@Override
	public void tick(MinecraftClient mc) {
		if (mc.player == null) {
			hasLast = false;
			joinedAt = System.currentTimeMillis();
			return;
		}
		if (hasLast) {
			double dx = mc.player.x - lastX, dz = mc.player.z - lastZ;
			double now = Math.sqrt(dx * dx + dz * dz) * 20; // per tick → per second, horizontal
			blocksPerSecond = blocksPerSecond * 0.7 + now * 0.3;
		}
		hasLast = true;
		lastX = mc.player.x;
		lastZ = mc.player.z;
	}

	private static Term.Line row(String key, String value, int color) {
		return new Term.Line().add(Term.pad(key, 7), Theme.MUTED).add(value, color);
	}

	private static String latency(MinecraftClient mc) {
		if (mc.player == null || mc.getNetworkHandler() == null) {
			return "-";
		}
		PlayerListEntry info = mc.getNetworkHandler().getPlayerListEntry(mc.player.getUuid());
		if (info == null || mc.isInSingleplayer()) {
			return "local";
		}
		return info.getLatency() + "ms";
	}

	/** Facing as N, NE, E... from the yaw (0 = south in Minecraft). */
	private static String compass(float yaw) {
		String[] dirs = {"S", "SW", "W", "NW", "N", "NE", "E", "SE"};
		int i = Math.floorMod(Math.round(yaw / 45f), 8);
		return dirs[i];
	}
}
