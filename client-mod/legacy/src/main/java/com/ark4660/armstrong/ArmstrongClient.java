package com.ark4660.armstrong;

import org.lwjgl.input.Keyboard;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import com.ark4660.armstrong.compat.Gfx;
import com.ark4660.armstrong.bench.Bench;
import com.ark4660.armstrong.bench.BenchRecorder;
import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.gui.ArmstrongScreen;
import com.ark4660.armstrong.home.Recent;
import com.ark4660.armstrong.module.ModuleManager;
import com.ark4660.armstrong.modules.ArmorHud;
import com.ark4660.armstrong.modules.ChatModules;
import com.ark4660.armstrong.modules.Crosshair;
import com.ark4660.armstrong.modules.EffectsHud;
import com.ark4660.armstrong.modules.InfoHuds;
import com.ark4660.armstrong.modules.Interface;
import com.ark4660.armstrong.modules.KeystrokesHud;
import com.ark4660.armstrong.modules.StatusHud;
import com.ark4660.armstrong.modules.ToggleSneak;
import com.ark4660.armstrong.modules.ToggleSprint;
import com.ark4660.armstrong.modules.ViewModules;
import com.ark4660.armstrong.modules.Zoom;
import com.ark4660.armstrong.modules.Freelook;
import com.ark4660.armstrong.cosmetics.Cosmetics;
import com.ark4660.armstrong.cosmetics.CosmeticsModule;
import com.ark4660.armstrong.server.ServerPolicy;
import com.ark4660.armstrong.waypoints.WaypointStore;
import com.ark4660.armstrong.waypoints.Waypoints;
import com.ark4660.armstrong.ui.Theme;

import net.fabricmc.api.ClientModInitializer;
import net.fabricmc.loader.api.FabricLoader;
import net.legacyfabric.fabric.api.client.event.lifecycle.v1.ClientLifecycleEvents;
import net.legacyfabric.fabric.api.client.event.lifecycle.v1.ClientTickEvents;
import net.legacyfabric.fabric.api.client.keybinding.v1.KeyBindingHelper;
import net.legacyfabric.fabric.api.client.networking.v1.ClientPlayConnectionEvents;
import net.legacyfabric.fabric.api.client.rendering.v1.HudRenderCallback;
import net.minecraft.client.MinecraftClient;
import net.minecraft.client.option.KeyBinding;

/**
 * The 1.8.9 entrypoint (Legacy Fabric). Same modules and menu as the modern
 * builds, minus what 1.8.9 can't do: replays (no Flashback), chunk borders,
 * HUD scaling and the TrueType font.
 */
public final class ArmstrongClient implements ClientModInitializer {
	public static final String MOD_ID = "armstrong";
	public static final Logger LOG = LogManager.getLogger("Armstrong");

	private static ModuleManager modules;
	private static Zoom zoom;
	private static Interface ui;
	private static KeyBinding menuKey;

	/** The Zoom module, for the FOV and scroll mixins. Null before init. */
	public static Zoom zoom() {
		return zoom;
	}

	public static ModuleManager modules() {
		return modules;
	}

	/** A module if it's switched on (for mixins); null before init or when off. */
	public static <M extends com.ark4660.armstrong.module.Module> M active(Class<M> type) {
		return modules == null ? null : modules.active(type);
	}

	/** The Interface module (look). Null before init. */
	public static Interface ui() {
		return ui;
	}

	public static KeyBinding menuKey() {
		return menuKey;
	}

	private static final String CATEGORY = "key.category.armstrong.armstrong";

	private static KeyBinding key(String name, int code) {
		return KeyBindingHelper.registerKeyBinding(new KeyBinding("key.armstrong." + name, code, CATEGORY));
	}

	@Override
	public void onInitializeClient() {
		// LWJGL 2 key codes (Keyboard.KEY_*); 0 is unbound.
		menuKey = key("menu", Keyboard.KEY_RSHIFT);
		KeyBinding zoomKey = key("zoom", Keyboard.KEY_C);

		modules = new ModuleManager(FabricLoader.getInstance().getConfigDir().resolve("armstrong.json"));
		KeyBinding snaplookKey = key("snaplook", Keyboard.KEY_V);
		KeyBinding stopwatchKey = key("stopwatch", Keyboard.KEY_NONE);
		KeyBinding freelookKey = key("freelook", Keyboard.KEY_LMENU);
		KeyBinding waypointAddKey = key("waypoint_add", Keyboard.KEY_B);
		KeyBinding waypointsKey = key("waypoints", Keyboard.KEY_N);
		KeyBinding[] autoTextKeys = {
			key("auto_text_1", Keyboard.KEY_NONE),
			key("auto_text_2", Keyboard.KEY_NONE),
			key("auto_text_3", Keyboard.KEY_NONE),
		};

		// Registration order is the menu's order: HUD, movement, view, chat, client.
		modules.register(new StatusHud());
		modules.register(new EffectsHud());
		modules.register(new ArmorHud());
		modules.register(new KeystrokesHud());
		modules.register(new InfoHuds.ItemsHud());
		modules.register(new InfoHuds.PvpInfo());
		modules.register(new InfoHuds.Stopwatch(stopwatchKey));
		modules.register(new InfoHuds.PackDisplay());
		modules.register(new Crosshair());
		modules.register(new ToggleSprint());
		modules.register(new ToggleSneak());
		zoom = modules.register(new Zoom(zoomKey));
		modules.register(new ViewModules.Snaplook(snaplookKey));
		modules.register(new Freelook(freelookKey));
		modules.register(new Waypoints(waypointAddKey, waypointsKey,
				new WaypointStore(FabricLoader.getInstance().getConfigDir().resolve("armstrong-waypoints.json"))));
		modules.register(new ViewModules.Fullbright());
		modules.register(new ViewModules.TimeChanger());
		modules.register(new ViewModules.WeatherChanger());
		modules.register(new ViewModules.LowFire());
		modules.register(new ViewModules.HideNameTags());
		modules.register(new ViewModules.Particles());
		modules.register(new ViewModules.Hitboxes());
		modules.register(new ChatModules.Chat());
		modules.register(new ChatModules.AutoText(autoTextKeys));
		modules.register(new ChatModules.AutoGG());
		modules.register(new CosmeticsModule());
		ui = modules.register(new Interface()); // last: it's settings, not a feature
		Theme.bind(ui);
		modules.load();
		if (BenchRecorder.requested()) {
			Bench.start();
		}
		Cosmetics.start(FabricLoader.getInstance().getConfigDir());
		Recent.init(FabricLoader.getInstance().getConfigDir().resolve("armstrong-recent.json"));
		// Remember the world or server for the home screen's continue entries.
		// Modules the server doesn't allow are off while you're on it.
		ServerPolicy policy = new ServerPolicy(FabricLoader.getInstance().getConfigDir().resolve("armstrong-servers.json"));
		ClientPlayConnectionEvents.DISCONNECT.register((handler, mc) -> policy.apply(modules, null));
		ClientPlayConnectionEvents.JOIN.register((handler, sender, mc) -> {
			policy.apply(modules, mc.getCurrentServerEntry() == null ? null : mc.getCurrentServerEntry().address);
			Recent.joined(mc);
		});

		ClientTickEvents.END_CLIENT_TICK.register(mc -> {
			while (menuKey.wasPressed()) {
				if (Mc.screen() == null) {
					Mc.open(new ArmstrongScreen(modules, menuKey));
				}
			}
			modules.tick(mc);
			if (Bench.enabled()) {
				Bench.tick(mc);
			}
		});
		HudRenderCallback.EVENT.register((mc, delta) -> {
			if (Bench.enabled()) {
				Bench.frame();
			}
			if (!mc.options.hudHidden) {
				Mc.partialTick = delta;
				modules.renderHud(new Gfx(), mc);
			}
		});
		ClientLifecycleEvents.CLIENT_STOPPING.register(mc -> modules.save());

		String version = FabricLoader.getInstance().getModContainer(MOD_ID)
				.map(c -> c.getMetadata().getVersion().getFriendlyString()).orElse("?");
		LOG.info("Armstrong {} ready with {} modules", version, modules.all().size());
	}
}
