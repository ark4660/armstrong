package com.ark4660.armstrong;

import com.ark4660.armstrong.compat.Mc;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import com.ark4660.armstrong.gui.ArmstrongScreen;
import com.ark4660.armstrong.home.Recent;
import com.ark4660.armstrong.replay.ReplayModule;
import com.ark4660.armstrong.module.ModuleManager;
import com.ark4660.armstrong.modules.ArmorHud;
import com.ark4660.armstrong.modules.ChatModules;
import com.ark4660.armstrong.modules.Crosshair;
import com.ark4660.armstrong.modules.HudTweaks;
import com.ark4660.armstrong.modules.InfoHuds;
import com.ark4660.armstrong.modules.ToggleSneak;
import com.ark4660.armstrong.modules.ViewModules;
import com.ark4660.armstrong.modules.EffectsHud;
import com.ark4660.armstrong.modules.Interface;
import com.ark4660.armstrong.modules.KeystrokesHud;
import com.ark4660.armstrong.modules.StatusHud;
import com.ark4660.armstrong.modules.ToggleSprint;
import com.ark4660.armstrong.modules.Zoom;
import com.ark4660.armstrong.modules.Freelook;
import com.ark4660.armstrong.server.ServerPolicy;
import com.ark4660.armstrong.waypoints.WaypointStore;
import com.ark4660.armstrong.waypoints.Waypoints;
import com.ark4660.armstrong.ui.Theme;
import com.mojang.blaze3d.platform.InputConstants;

import net.fabricmc.api.ClientModInitializer;
import net.fabricmc.fabric.api.client.event.lifecycle.v1.ClientLifecycleEvents;
import net.fabricmc.fabric.api.client.event.lifecycle.v1.ClientTickEvents;
import net.fabricmc.fabric.api.client.message.v1.ClientReceiveMessageEvents;
import net.fabricmc.fabric.api.client.networking.v1.ClientPlayConnectionEvents;
import net.fabricmc.fabric.api.event.player.AttackEntityCallback;
import net.minecraft.world.InteractionResult;
//? if >=26 {
import net.fabricmc.fabric.api.client.keymapping.v1.KeyMappingHelper;
import net.fabricmc.fabric.api.client.rendering.v1.hud.HudElementRegistry;
import net.fabricmc.fabric.api.client.rendering.v1.hud.VanillaHudElements;
//?} else {
/*import net.fabricmc.fabric.api.client.keybinding.v1.KeyBindingHelper;
import net.fabricmc.fabric.api.client.rendering.v1.HudRenderCallback;
*///?}
import net.fabricmc.loader.api.FabricLoader;
import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;
import net.minecraft.resources.Identifier;

public final class ArmstrongClient implements ClientModInitializer {
	public static final String MOD_ID = "armstrong";
	public static final Logger LOG = LoggerFactory.getLogger("Armstrong");

	private static ModuleManager modules;
	private static Zoom zoom;
	private static Interface ui;
	private static KeyMapping menuKey;

	/** The Zoom module, for the camera and mouse mixins. Null before init. */
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

	/** The Interface module (look and home screen). Null before init. */
	public static Interface ui() {
		return ui;
	}

	public static KeyMapping menuKey() {
		return menuKey;
	}

	public static Identifier id(String path) {
		return Identifier.fromNamespaceAndPath(MOD_ID, path);
	}

	// The controls-screen group for Armstrong's keys: a registered category in
	// 26.x, a translation key before.
	//? if >=26 {
	private static KeyMapping.Category category;
	//?} else {
	/*private static final String category = "key.category.armstrong.armstrong";
	*///?}

	private static KeyMapping key(String name, int code) {
		//? if >=26 {
		return KeyMappingHelper.registerKeyMapping(new KeyMapping("key.armstrong." + name, code, category));
		//?} else {
		/*return KeyBindingHelper.registerKeyBinding(new KeyMapping("key.armstrong." + name, code, category));
		*///?}
	}

	@Override
	public void onInitializeClient() {
		// Key codes differ by version (SDL scancodes in 26.x, GLFW keys before):
		// use InputConstants names, never raw numbers.
		//? if >=26
		category = KeyMapping.Category.register(id(MOD_ID));
		menuKey = key("menu", InputConstants.KEY_RSHIFT);
		KeyMapping zoomKey = key("zoom", InputConstants.KEY_C);

		modules = new ModuleManager(FabricLoader.getInstance().getConfigDir().resolve("armstrong.json"));
		KeyMapping snaplookKey = key("snaplook", InputConstants.KEY_V);
		KeyMapping replayKey = key("replay_record", InputConstants.UNKNOWN.getValue());
		KeyMapping stopwatchKey = key("stopwatch", InputConstants.UNKNOWN.getValue());
		KeyMapping freelookKey = key("freelook", InputConstants.KEY_LALT);
		KeyMapping waypointAddKey = key("waypoint_add", InputConstants.KEY_B);
		KeyMapping waypointsKey = key("waypoints", InputConstants.KEY_N);
		KeyMapping[] autoTextKeys = {
			key("auto_text_1", InputConstants.UNKNOWN.getValue()),
			key("auto_text_2", InputConstants.UNKNOWN.getValue()),
			key("auto_text_3", InputConstants.UNKNOWN.getValue()),
		};

		// Registration order is the menu's order: HUD, movement, view, chat, client.
		modules.register(new StatusHud());
		modules.register(new EffectsHud());
		modules.register(new ArmorHud());
		modules.register(new KeystrokesHud());
		modules.register(new InfoHuds.ItemsHud());
		InfoHuds.PvpInfo pvpInfo = modules.register(new InfoHuds.PvpInfo());
		modules.register(new InfoHuds.Stopwatch(stopwatchKey));
		modules.register(new InfoHuds.PackDisplay());
		modules.register(new Crosshair());
		modules.register(new HudTweaks());
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
		modules.register(new ViewModules.Particles());
		modules.register(new ViewModules.Hitboxes());
		modules.register(new ViewModules.ChunkBorders());
		modules.register(new ChatModules.Chat());
		modules.register(new ChatModules.AutoText(autoTextKeys));
		ChatModules.AutoGG autoGG = modules.register(new ChatModules.AutoGG());
		ReplayModule replay = modules.register(new ReplayModule(replayKey));
		ui = modules.register(new Interface()); // last: it's settings, not a feature
		Theme.bind(ui);
		modules.load();
		Recent.init(FabricLoader.getInstance().getConfigDir().resolve("armstrong-recent.json"));
		// Remember the world or server for the home screen's continue entries.
		// Modules the server doesn't allow are off while you're on it.
		ServerPolicy policy = new ServerPolicy(FabricLoader.getInstance().getConfigDir().resolve("armstrong-servers.json"));
		ClientPlayConnectionEvents.DISCONNECT.register((handler, mc) -> policy.apply(modules, null));
		ClientReceiveMessageEvents.GAME.register((message, overlay) -> {
			if (!overlay && autoGG.isActive()) {
				autoGG.onServerMessage(message);
			}
		});
		ClientPlayConnectionEvents.JOIN.register((handler, sender, mc) -> {
			policy.apply(modules, mc.getCurrentServer() == null ? null : mc.getCurrentServer().ip);
			Recent.joined(mc);
			replay.onJoin();
		});

		ClientTickEvents.END_CLIENT_TICK.register(mc -> {
			while (menuKey.consumeClick()) {
				if (Mc.screen() == null) {
					Mc.open(new ArmstrongScreen(modules, menuKey));
				}
			}
			modules.tick(mc);
		});
		//? if >=26 {
		HudElementRegistry.attachElementBefore(VanillaHudElements.CHAT, id("modules"),
				(g, delta) -> modules.renderHud(g, Minecraft.getInstance()));
		HudElementRegistry.replaceElement(VanillaHudElements.MOB_EFFECTS,
				vanilla -> (g, delta) -> HudHooks.effects(g, () -> vanilla.extractRenderState(g, delta)));
		HudElementRegistry.replaceElement(VanillaHudElements.CROSSHAIR,
				vanilla -> (g, delta) -> HudHooks.crosshair(g, () -> vanilla.extractRenderState(g, delta)));
		HudElementRegistry.replaceElement(VanillaHudElements.SCOREBOARD,
				vanilla -> (g, delta) -> HudHooks.scoreboard(g, () -> vanilla.extractRenderState(g, delta)));
		HudElementRegistry.replaceElement(VanillaHudElements.BOSS_BAR,
				vanilla -> (g, delta) -> HudHooks.bossBar(g, () -> vanilla.extractRenderState(g, delta)));
		HudElementRegistry.replaceElement(VanillaHudElements.TITLE_AND_SUBTITLE,
				vanilla -> (g, delta) -> HudHooks.titles(g, () -> vanilla.extractRenderState(g, delta)));
		//?} else {
		/*// The vanilla elements go through HudHooks from HudMixin and BossBarMixin.
		// The HUD is drawn in depth layers (200 apart): draw in front of them all.
		HudRenderCallback.EVENT.register((g, delta) -> {
			g.pose().pushPose();
			g.pose().translate(0f, 0f, 4000f);
			modules.renderHud(g, Minecraft.getInstance());
			g.pose().popPose();
		});
		*///?}
		// Hits you make, for combo and reach.
		AttackEntityCallback.EVENT.register((player, level, hand, entity, hit) -> {
			if (level.isClientSide() && pvpInfo.isActive()) {
				pvpInfo.onAttack(Minecraft.getInstance(), entity);
			}
			return InteractionResult.PASS;
		});
		ClientLifecycleEvents.CLIENT_STOPPING.register(mc -> modules.save());

		String version = FabricLoader.getInstance().getModContainer(MOD_ID)
				.map(c -> c.getMetadata().getVersion().getFriendlyString()).orElse("?");
		LOG.info("Armstrong {} ready with {} modules", version, modules.all().size());
	}
}
