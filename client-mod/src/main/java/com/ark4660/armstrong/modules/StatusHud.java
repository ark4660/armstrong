package com.ark4660.armstrong.modules;

import java.util.ArrayList;
import java.util.List;

import com.ark4660.armstrong.input.ClickCounter;
import com.ark4660.armstrong.module.HudModule;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.fabricmc.loader.api.FabricLoader;
import net.minecraft.client.Minecraft;
import net.minecraft.client.multiplayer.PlayerInfo;

/** The status block: fps, ping, coordinates, facing and clicks per second. */
public final class StatusHud extends HudModule.Text {
	private final Setting.Bool header = add(new Setting.Bool("header", "header", true));
	private final Setting.Bool fps = add(new Setting.Bool("fps", "fps", true));
	private final Setting.Bool ping = add(new Setting.Bool("ping", "ping", true));
	private final Setting.Bool coords = add(new Setting.Bool("coords", "coords", true));
	private final Setting.Bool facing = add(new Setting.Bool("facing", "facing", true));
	private final Setting.Bool cps = add(new Setting.Bool("cps", "cps", true));

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
			out.add(new Term.Line().add(Term.pad("cps", 6), Theme.MUTED)
					.add(String.valueOf(ClickCounter.left()), accent).add(" | ", Theme.MUTED)
					.add(String.valueOf(ClickCounter.right()), accent));
		}
		return out;
	}

	private static Term.Line row(String key, String value, int color) {
		return new Term.Line().add(Term.pad(key, 6), Theme.MUTED).add(value, color);
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
