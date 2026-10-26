package com.ark4660.armstrong.waypoints;

import java.io.IOException;
import java.lang.reflect.Type;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.reflect.TypeToken;

import com.ark4660.armstrong.ArmstrongClient;

import net.minecraft.client.Minecraft;
import net.minecraft.client.multiplayer.ServerData;
import net.minecraft.client.server.IntegratedServer;
import net.minecraft.world.level.storage.LevelResource;

/**
 * Waypoints for every world and server, in config/armstrong-waypoints.json,
 * keyed by "world:<save folder>" or "server:<address>".
 */
public final class WaypointStore {
	private static final Gson GSON = new GsonBuilder().setPrettyPrinting().create();
	private static final Type TYPE = new TypeToken<LinkedHashMap<String, List<Waypoint>>>() {}.getType();

	private final Path file;
	private Map<String, List<Waypoint>> all;

	public WaypointStore(Path file) {
		this.file = file;
	}

	private Map<String, List<Waypoint>> all() {
		if (all == null) {
			all = new LinkedHashMap<>();
			if (Files.exists(file)) {
				try {
					Map<String, List<Waypoint>> read = GSON.fromJson(Files.readString(file), TYPE);
					if (read != null) {
						all = read;
					}
				} catch (IOException | RuntimeException e) {
					ArmstrongClient.LOG.warn("Couldn't read {}: {}", file, e.toString());
				}
			}
		}
		return all;
	}

	/** The key of the world or server you're in, or null on the title screen. */
	public static String currentKey(Minecraft mc) {
		IntegratedServer local = mc.getSingleplayerServer();
		if (local != null) {
			Path root = local.getWorldPath(LevelResource.ROOT).toAbsolutePath().normalize();
			return "world:" + root.getFileName();
		}
		ServerData sd = mc.getCurrentServer();
		return sd == null ? null : "server:" + sd.ip.toLowerCase(Locale.ROOT);
	}

	/** This world's waypoints (a live list: change it, then save). */
	public List<Waypoint> list(String key) {
		return all().computeIfAbsent(key, k -> new ArrayList<>());
	}

	public void save() {
		if (all == null) {
			return;
		}
		all.values().removeIf(List::isEmpty);
		try {
			Files.createDirectories(file.getParent());
			Path tmp = file.resolveSibling(file.getFileName() + ".tmp");
			Files.writeString(tmp, GSON.toJson(all, TYPE));
			Files.move(tmp, file, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
		} catch (IOException e) {
			ArmstrongClient.LOG.warn("Couldn't save {}: {}", file, e.toString());
		}
	}
}
