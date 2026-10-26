package com.ark4660.armstrong.module;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import com.ark4660.armstrong.compat.Gfx;

import net.minecraft.client.MinecraftClient;

/** Holds every module and saves their state to config/armstrong.json. */
public final class ModuleManager {
	private static final Logger LOG = LogManager.getLogger("Armstrong");
	private static final Gson GSON = new GsonBuilder().setPrettyPrinting().create();

	private final List<Module> modules = new ArrayList<>();
	private final Path configFile;

	public ModuleManager(Path configFile) {
		this.configFile = configFile;
	}

	public <M extends Module> M register(M module) {
		modules.add(module);
		return module;
	}

	/** The registered module of a type, or null. */
	public <M extends Module> M get(Class<M> type) {
		for (Module m : modules) {
			if (type.isInstance(m)) {
				return type.cast(m);
			}
		}
		return null;
	}

	/** The module of a type if it's registered and switched on, else null. */
	public <M extends Module> M active(Class<M> type) {
		M m = get(type);
		return m != null && m.isActive() ? m : null;
	}

	public List<Module> all() {
		return Collections.unmodifiableList(modules);
	}

	public void tick(MinecraftClient mc) {
		for (Module m : modules) {
			if (m.isActive()) {
				m.tick(mc);
			}
		}
	}

	/** Draws HUD modules, from Legacy Fabric's HUD callback (hidden with F1). */
	/** Set while the HUD editor is open: it draws the panels itself, on top. */
	private boolean editorOpen;

	public void setEditorOpen(boolean open) {
		editorOpen = open;
	}

	public void renderHud(Gfx g, MinecraftClient mc) {
		if (!editorOpen) {
			renderHud(g, mc, false);
		}
	}

	/** Draws HUD modules; editing shows empty panels too, so they can be moved. */
	public void renderHud(Gfx g, MinecraftClient mc, boolean editing) {
		for (Module m : modules) {
			if (m instanceof HudOverlay o && m.isActive()) {
				o.drawOverlay(g, mc); // under the panels
			}
		}
		// Panels in the same corner stack, in menu order, so none overlap.
		java.util.Map<String, Integer> used = new java.util.HashMap<>();
		for (Module m : modules) {
			if (m instanceof HudModule hud && hud.isActive()) {
				int before = used.getOrDefault(hud.anchor(), 0);
				used.put(hud.anchor(), before + hud.render(g, mc, before, editing));
			}
		}
		renderIndicators(g);
	}

	/** Status tags like [sprint: toggled] in a row, centred above the hotbar and health. */
	private void renderIndicators(Gfx g) {
		List<Term.Line> tags = new ArrayList<>();
		for (Module m : modules) {
			if (m instanceof Indicator ind && m.isActive() && ind.indicatorLabel() != null) {
				tags.add(new Term.Line().add("[" + ind.indicatorLabel() + ": ", Theme.TEXT)
						.add(ind.indicatorValue(), m instanceof com.ark4660.armstrong.modules.Zoom ? Theme.AMBER : Theme.accent())
						.add("]", Theme.TEXT));
			}
		}
		if (tags.isEmpty()) {
			return;
		}
		int gap = 6, total = -gap;
		for (Term.Line t : tags) {
			total += t.width() + 8 + gap;
		}
		int x = (g.guiWidth() - total) / 2;
		int y = g.guiHeight() - 66;
		for (Term.Line t : tags) {
			int w = t.width() + 8;
			Term.panel(g, x, y, w, Term.LINE + 3, Theme.PANEL, Theme.BORDER);
			t.draw(g, x + 4, y + 2);
			x += w + gap;
		}
	}

	/** Loads saved state; a missing or broken file leaves the defaults. */
	public void load() {
		if (!Files.exists(configFile)) {
			return;
		}
		try {
			JsonObject root = new JsonParser().parse(Files.readString(configFile)).getAsJsonObject();
			JsonObject saved = root.getAsJsonObject("modules");
			if (saved == null) {
				return;
			}
			for (Module m : modules) {
				JsonObject entry = saved.getAsJsonObject(m.id);
				if (entry == null) {
					continue;
				}
				JsonElement enabled = entry.get("enabled");
				if (enabled != null && enabled.isJsonPrimitive()) {
					m.setEnabled(enabled.getAsBoolean());
				}
				JsonObject settings = entry.getAsJsonObject("settings");
				if (settings != null) {
					for (Setting<?> s : m.settings()) {
						JsonElement v = settings.get(s.id);
						if (v != null) {
							s.fromJson(v);
						}
					}
				}
			}
		} catch (IOException | RuntimeException e) {
			LOG.warn("Couldn't read {}, using defaults: {}", configFile, e.toString());
		}
	}

	public void save() {
		JsonObject saved = new JsonObject();
		for (Module m : modules) {
			JsonObject settings = new JsonObject();
			for (Setting<?> s : m.settings()) {
				settings.add(s.id, s.toJson());
			}
			JsonObject entry = new JsonObject();
			entry.addProperty("enabled", m.isEnabled());
			entry.add("settings", settings);
			saved.add(m.id, entry);
		}
		JsonObject root = new JsonObject();
		root.addProperty("version", 1);
		root.add("modules", saved);
		try {
			Files.createDirectories(configFile.getParent());
			Path tmp = configFile.resolveSibling(configFile.getFileName() + ".tmp");
			Files.writeString(tmp, GSON.toJson(root));
			Files.move(tmp, configFile, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
		} catch (IOException e) {
			LOG.warn("Couldn't save {}: {}", configFile, e.toString());
		}
	}

	/** Modules by category, in registration order. */
	public Map<Category, List<Module>> byCategory() {
		Map<Category, List<Module>> out = new java.util.EnumMap<>(Category.class);
		for (Module m : modules) {
			out.computeIfAbsent(m.category, c -> new ArrayList<>()).add(m);
		}
		return out;
	}
}
