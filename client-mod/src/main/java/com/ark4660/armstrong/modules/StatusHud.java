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
import net.minecraft.client.Minecraft;
import net.minecraft.client.multiplayer.PlayerInfo;
import net.minecraft.world.phys.Vec3;

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
	private Vec3 lastPos;
	private double blocksPerSecond;

	private final String version = FabricLoader.getInstance().getModContainer("armstrong")
			.map(c -> c.getMetadata().getVersion().getFriendlyString()).orElse("");

	public StatusHud() {
		super("status", "Status", "fps, ping, coordinates, facing and clicks per second.", true, "Top left");
	}

	@Override
	protected List<Term.Line> lines(Minecraft mc) {
		int accent = Theme.accent();
		List<Term.Line> out = new ArrayList<>();
		if (header.get()) {
			out.add(new Term.Line().add("armstrong " + version + " ", accent).add("─".repeat(8), Theme.MUTED));
		}
		if (fps.get()) {
			out.add(row("fps", String.valueOf(mc.getFps()), accent));
		}
		if (ping.get()) {
			out.add(row("ping", latency(mc), accent));
		}
		if (mc.player != null) {
			if (coords.get()) {
				out.add(row("xyz", String.format("%d %d %d",
						(int) Math.floor(mc.player.getX()), (int) Math.floor(mc.player.getY()), (int) Math.floor(mc.player.getZ())), Theme.TEXT));
			}
			if (facing.get()) {
				out.add(row("dir", compass(mc.player.getYRot()), Theme.TEXT));
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
				out.add(row("food", mc.player.getFoodData().getFoodLevel() + " · sat " + String.format("%.1f", mc.player.getFoodData().getSaturationLevel()), Theme.TEXT));
			}
			if (biome.get() && mc.level != null) {
				String name = mc.level.getBiome(mc.player.blockPosition()).unwrapKey()
						//? if >=1.21.11 {
						.map(k -> k.identifier().getPath()).orElse("?");
						//?} else {
						/*.map(k -> k.location().getPath()).orElse("?");
						*///?}
				out.add(row("biome", name, Theme.TEXT));
			}
		}
		if (memory.get()) {
			Runtime rt = Runtime.getRuntime();
			long used = (rt.totalMemory() - rt.freeMemory()) >> 20, max = rt.maxMemory() >> 20;
			out.add(row("mem", used + "/" + max + " MB", used * 10 > max * 9 ? Theme.AMBER : Theme.TEXT));
		}
		if (server.get()) {
			var sd = mc.getCurrentServer();
			out.add(row("server", sd != null ? sd.ip : mc.hasSingleplayerServer() ? "singleplayer" : "-", Theme.TEXT));
		}
		return out;
	}

	private static final DateTimeFormatter CLOCK = DateTimeFormatter.ofPattern("HH:mm");

	@Override
	public void tick(Minecraft mc) {
		if (mc.player == null) {
			lastPos = null;
			joinedAt = System.currentTimeMillis();
			return;
		}
		Vec3 pos = mc.player.position();
		if (lastPos != null) {
			double dx = pos.x - lastPos.x, dz = pos.z - lastPos.z;
			double now = Math.sqrt(dx * dx + dz * dz) * 20; // per tick → per second, horizontal
			blocksPerSecond = blocksPerSecond * 0.7 + now * 0.3;
		}
		lastPos = pos;
	}

	private static Term.Line row(String key, String value, int color) {
		return new Term.Line().add(Term.pad(key, 7), Theme.MUTED).add(value, color);
	}

	private static String latency(Minecraft mc) {
		if (mc.player == null || mc.getConnection() == null) {
			return "-";
		}
		PlayerInfo info = mc.getConnection().getPlayerInfo(mc.player.getUUID());
		if (info == null || mc.hasSingleplayerServer()) {
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
