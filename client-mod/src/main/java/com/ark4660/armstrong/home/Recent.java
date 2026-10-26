package com.ark4660.armstrong.home;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;

import com.ark4660.armstrong.ArmstrongClient;

import net.minecraft.client.Minecraft;
import net.minecraft.client.multiplayer.ServerData;
import net.minecraft.client.server.IntegratedServer;
import net.minecraft.world.level.storage.LevelResource;

/** The last world and server you played, for the home screen's "continue". */
public final class Recent {
	private static final Gson GSON = new GsonBuilder().setPrettyPrinting().create();

	public static final class Entry {
		public String name;
		public String id; // world folder, or server address
		public long time; // epoch millis

		Entry(String name, String id) {
			this.name = name;
			this.id = id;
			this.time = System.currentTimeMillis();
		}
	}

	public Entry world;
	public Entry server;

	private static Recent current;
	private static Path file;

	public static Recent get() {
		if (current == null) {
			current = load();
		}
		return current;
	}

	public static void init(Path configFile) {
		file = configFile;
	}

	private static Recent load() {
		if (file != null && Files.exists(file)) {
			try {
				Recent r = GSON.fromJson(Files.readString(file), Recent.class);
				if (r != null) {
					return r;
				}
			} catch (IOException | RuntimeException e) {
				ArmstrongClient.LOG.warn("Couldn't read {}: {}", file, e.toString());
			}
		}
		return new Recent();
	}

	/** Records what was just joined (called when a world or server is ready). */
	public static void joined(Minecraft mc) {
		Recent r = get();
		IntegratedServer local = mc.getSingleplayerServer();
		if (local != null) {
			Path root = local.getWorldPath(LevelResource.ROOT).toAbsolutePath().normalize();
			r.world = new Entry(local.getWorldData().getLevelName(), root.getFileName().toString());
		} else {
			ServerData sd = mc.getCurrentServer();
			if (sd == null || sd.type() == ServerData.Type.REALM) {
				return;
			}
			r.server = new Entry(sd.name, sd.ip);
		}
		save(r);
	}

	private static void save(Recent r) {
		if (file == null) {
			return;
		}
		try {
			Files.createDirectories(file.getParent());
			Files.writeString(file, GSON.toJson(r));
		} catch (IOException e) {
			ArmstrongClient.LOG.warn("Couldn't save {}: {}", file, e.toString());
		}
	}

	/** "2h ago"-style age. */
	public static String ago(long time) {
		long s = Math.max(0, (System.currentTimeMillis() - time) / 1000);
		if (s < 60) {
			return "just now";
		}
		if (s < 3600) {
			return s / 60 + "m ago";
		}
		if (s < 86400) {
			return s / 3600 + "h ago";
		}
		return s / 86400 + "d ago";
	}
}
