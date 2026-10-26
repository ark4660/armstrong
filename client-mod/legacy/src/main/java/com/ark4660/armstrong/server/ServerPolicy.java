package com.ark4660.armstrong.server;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.google.gson.JsonPrimitive;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.ModuleManager;

/**
 * Modules a server doesn't allow, switched off while you're on it (the way
 * Lunar and Badlion respect server rules, so players don't get banned).
 * Rules live in config/armstrong-servers.json: a host (which also covers its
 * subdomains) and the module ids it blocks. The file starts with the
 * built-in rules; players and server owners can add their own.
 */
public final class ServerPolicy {
	private static final Gson GSON = new GsonBuilder().setPrettyPrinting().create();

	/** Built-in rules, written to the file the first time. */
	private static final Map<String, List<String>> DEFAULTS = Map.of(
			// Hypixel asked clients to disable freelook (it shows what's around corners).
			"hypixel.net", List.of("freelook"));

	private final Path file;
	private Map<String, List<String>> rules = new LinkedHashMap<>();
	private String current; // host the rules were applied for

	public ServerPolicy(Path file) {
		this.file = file;
		load();
	}

	private void load() {
		rules = new LinkedHashMap<>(DEFAULTS);
		if (!Files.exists(file)) {
			save();
			return;
		}
		try {
			JsonObject root = new JsonParser().parse(Files.readString(file)).getAsJsonObject();
			JsonObject servers = root.getAsJsonObject("servers");
			if (servers == null) {
				return;
			}
			rules = new LinkedHashMap<>();
			for (Map.Entry<String, JsonElement> e : servers.entrySet()) {
				List<String> ids = new ArrayList<>();
				for (JsonElement id : e.getValue().getAsJsonArray()) {
					ids.add(id.getAsString());
				}
				rules.put(e.getKey().toLowerCase(Locale.ROOT), ids);
			}
		} catch (IOException | RuntimeException e) {
			ArmstrongClient.LOG.warn("Couldn't read {}, using the built-in server rules: {}", file, e.toString());
		}
	}

	private void save() {
		JsonObject servers = new JsonObject();
		rules.forEach((host, ids) -> {
			JsonArray arr = new JsonArray();
			ids.forEach(id -> arr.add(new JsonPrimitive(id))); // Gson 2.2: no add(String)
			servers.add(host, arr);
		});
		JsonObject root = new JsonObject();
		root.addProperty("_help", "Modules each server blocks. A host also covers its subdomains. Module ids are shown in the Armstrong menu.");
		root.add("servers", servers);
		try {
			Files.createDirectories(file.getParent());
			Files.writeString(file, GSON.toJson(root));
		} catch (IOException e) {
			ArmstrongClient.LOG.warn("Couldn't write {}: {}", file, e.toString());
		}
	}

	/** The rule's host that matches an address (host[:port]), or null. */
	String match(String address) {
		String host = host(address);
		for (String rule : rules.keySet()) {
			if (host.equals(rule) || host.endsWith("." + rule)) {
				return rule;
			}
		}
		return null;
	}

	static String host(String address) {
		String a = address.trim().toLowerCase(Locale.ROOT);
		if (a.startsWith("[")) { // [ipv6]:port
			int end = a.indexOf(']');
			return end > 0 ? a.substring(1, end) : a;
		}
		int colon = a.indexOf(':');
		if (colon >= 0 && a.indexOf(':', colon + 1) < 0) {
			a = a.substring(0, colon);
		}
		return a.endsWith(".") ? a.substring(0, a.length() - 1) : a;
	}

	/** Applies the rules for the server just joined; null (singleplayer) lifts them. */
	public void apply(ModuleManager modules, String address) {
		load(); // pick up edits without a restart
		String rule = address == null ? null : match(address);
		List<String> blocked = rule == null ? List.of() : rules.get(rule);
		for (Module m : modules.all()) {
			m.setBlocked(blocked.contains(m.id) ? rule : null);
		}
		if (rule != null && !blocked.isEmpty() && !rule.equals(current)) {
			ArmstrongClient.LOG.info("{} blocks: {}", rule, String.join(", ", blocked));
		}
		current = rule;
	}
}
