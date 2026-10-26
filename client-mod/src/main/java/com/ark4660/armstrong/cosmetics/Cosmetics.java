package com.ark4660.armstrong.cosmetics;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.time.Duration;
import java.util.HashMap;
import java.util.HexFormat;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.compat.Mc;

import net.minecraft.resources.Identifier;

/**
 * Capes and badges from the signed cosmetics list in the repository. It's
 * downloaded at start and every 15 minutes, checked against CosmeticsKey,
 * and cached (with the cape images) so cosmetics also show offline. Only
 * Armstrong players see them.
 */
public final class Cosmetics {
	private static final String BASE_URL = System.getProperty("armstrong.cosmetics.url",
			"https://raw.githubusercontent.com/ark4660/armstrong/main/cosmetics/");
	private static final long REFRESH_MINUTES = 15;
	private static final int MAX_CAPE_BYTES = 1 << 20;

	/** A badge as drawn: the symbol and its colour (RGB). */
	public record Badge(String symbol, int rgb) {}

	private record Entry(String cape, Badge badge) {}

	private static final HttpClient HTTP = HttpClient.newBuilder()
			.connectTimeout(Duration.ofSeconds(10)).followRedirects(HttpClient.Redirect.NORMAL).build();

	private static Path cacheDir;
	private static volatile Map<UUID, Entry> players = Map.of();
	/** Cape id → its texture, once loaded on the render thread. */
	private static final Map<String, Identifier> textures = new ConcurrentHashMap<>();
	/** Cape id → sha256 of the loaded texture (a changed cape gets a new texture). */
	private static final Map<String, String> loaded = new ConcurrentHashMap<>();

	private Cosmetics() {}

	/** Starts loading: the cached list right away, then the latest one in the background. */
	public static void start(Path configDir) {
		cacheDir = configDir.resolve("armstrong-cosmetics");
		ScheduledExecutorService ex = Executors.newSingleThreadScheduledExecutor(r -> {
			Thread t = new Thread(r, "Armstrong cosmetics");
			t.setDaemon(true);
			return t;
		});
		ex.execute(() -> load(false));
		ex.scheduleWithFixedDelay(() -> load(true), 0, REFRESH_MINUTES, TimeUnit.MINUTES);
	}

	/** The cape texture of a player, or null. */
	public static Identifier cape(UUID player) {
		Entry e = players.get(player);
		return e == null || e.cape == null ? null : textures.get(e.cape);
	}

	/** The badge of a player, or null. */
	public static Badge badge(UUID player) {
		Entry e = players.get(player);
		return e == null ? null : e.badge;
	}

	private static void load(boolean download) {
		try {
			byte[] data;
			String sig;
			if (download) {
				data = get("cosmetics.json", 1 << 22);
				sig = new String(get("cosmetics.json.sig", 1024), StandardCharsets.UTF_8);
			} else {
				Path f = cacheDir.resolve("cosmetics.json");
				if (!Files.exists(f)) {
					return;
				}
				data = Files.readAllBytes(f);
				sig = Files.readString(cacheDir.resolve("cosmetics.json.sig"));
			}
			if (!CosmeticsKey.verify(data, sig)) {
				ArmstrongClient.LOG.warn("Cosmetics list signature doesn't match; ignoring it");
				return;
			}
			apply(data);
			if (download) {
				Files.createDirectories(cacheDir);
				Files.write(cacheDir.resolve("cosmetics.json"), data);
				Files.writeString(cacheDir.resolve("cosmetics.json.sig"), sig);
			}
		} catch (IOException | RuntimeException | InterruptedException e) {
			if (download) {
				ArmstrongClient.LOG.info("Couldn't refresh cosmetics: {}", e.toString());
			}
		}
	}

	private static void apply(byte[] data) throws IOException, InterruptedException {
		JsonObject root = JsonParser.parseString(new String(data, StandardCharsets.UTF_8)).getAsJsonObject();
		Map<String, Badge> badges = new HashMap<>();
		for (Map.Entry<String, JsonElement> b : obj(root, "badges").entrySet()) {
			JsonObject o = b.getValue().getAsJsonObject();
			badges.put(b.getKey(), new Badge(o.get("symbol").getAsString(),
					Integer.parseInt(o.get("color").getAsString().substring(1), 16)));
		}
		// Capes: make sure each image is cached and matches its hash.
		Map<String, String> capeHashes = new HashMap<>();
		for (Map.Entry<String, JsonElement> c : obj(root, "capes").entrySet()) {
			JsonObject o = c.getValue().getAsJsonObject();
			String file = o.get("file").getAsString();
			String sha = o.get("sha256").getAsString();
			if (!file.matches("capes/[a-z0-9_-]+\\.png")) {
				continue;
			}
			Path cached = cacheDir.resolve(file);
			if (!Files.exists(cached) || !sha256(Files.readAllBytes(cached)).equals(sha)) {
				byte[] png = get(file, MAX_CAPE_BYTES);
				if (!sha256(png).equals(sha)) {
					ArmstrongClient.LOG.warn("Cape {} doesn't match the signed list; skipped", c.getKey());
					continue;
				}
				Files.createDirectories(cached.getParent());
				Files.write(cached, png);
			}
			capeHashes.put(c.getKey(), sha);
		}
		Map<UUID, Entry> next = new HashMap<>();
		for (Map.Entry<String, JsonElement> p : obj(root, "players").entrySet()) {
			JsonObject o = p.getValue().getAsJsonObject();
			String cape = o.has("cape") ? o.get("cape").getAsString() : null;
			Badge badge = o.has("badge") ? badges.get(o.get("badge").getAsString()) : null;
			next.put(uuid(p.getKey()), new Entry(capeHashes.containsKey(cape) ? cape : null, badge));
		}
		if (!next.equals(players)) {
			ArmstrongClient.LOG.info("Cosmetics: {} players, {} capes", next.size(), capeHashes.size());
		}
		players = next;
		// Textures are made on the render thread.
		for (Map.Entry<String, String> c : capeHashes.entrySet()) {
			if (!c.getValue().equals(loaded.get(c.getKey()))) {
				byte[] png = Files.readAllBytes(cacheDir.resolve("capes/" + c.getKey() + ".png"));
				Mc.onRenderThread(() -> {
					Identifier id = ArmstrongClient.id("cosmetics/cape/" + c.getKey() + "_" + c.getValue().substring(0, 8));
					if (Mc.registerTexture(id, png)) {
						textures.put(c.getKey(), id);
						loaded.put(c.getKey(), c.getValue());
					}
				});
			}
		}
	}

	private static JsonObject obj(JsonObject root, String key) {
		JsonElement e = root.get(key);
		return e != null && e.isJsonObject() ? e.getAsJsonObject() : new JsonObject();
	}

	private static UUID uuid(String noDashes) {
		return new UUID(Long.parseUnsignedLong(noDashes.substring(0, 16), 16), Long.parseUnsignedLong(noDashes.substring(16), 16));
	}

	private static byte[] get(String path, int maxBytes) throws IOException, InterruptedException {
		HttpRequest req = HttpRequest.newBuilder(URI.create(BASE_URL + path))
				.timeout(Duration.ofSeconds(20)).header("User-Agent", "Armstrong (github.com/ark4660/armstrong)").build();
		HttpResponse<byte[]> resp = HTTP.send(req, HttpResponse.BodyHandlers.ofByteArray());
		if (resp.statusCode() != 200) {
			throw new IOException(path + ": HTTP " + resp.statusCode());
		}
		if (resp.body().length > maxBytes) {
			throw new IOException(path + ": too large");
		}
		return resp.body();
	}

	private static String sha256(byte[] data) {
		try {
			return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(data));
		} catch (java.security.NoSuchAlgorithmException e) {
			throw new IllegalStateException(e);
		}
	}
}
