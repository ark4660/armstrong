package com.ark4660.armstrong;

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
import com.ark4660.armstrong.ui.Theme;
import com.mojang.blaze3d.platform.InputConstants;

import net.fabricmc.api.ClientModInitializer;
import net.fabricmc.fabric.api.client.event.lifecycle.v1.ClientLifecycleEvents;
import net.fabricmc.fabric.api.client.event.lifecycle.v1.ClientTickEvents;
import net.fabricmc.fabric.api.client.keymapping.v1.KeyMappingHelper;
import net.fabricmc.fabric.api.client.networking.v1.ClientPlayConnectionEvents;
import net.fabricmc.fabric.api.client.rendering.v1.hud.HudElementRegistry;
import net.fabricmc.fabric.api.event.player.AttackEntityCallback;
import net.minecraft.world.InteractionResult;
import net.fabricmc.fabric.api.client.rendering.v1.hud.VanillaHudElements;
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

	private static KeyMapping key(String name, int code, KeyMapping.Category category) {
		return KeyMappingHelper.registerKeyMapping(new KeyMapping("key.armstrong." + name, code, category));
	}

	@Override
	public void onInitializeClient() {
		// 26.x key codes are SDL scancodes: use InputConstants names, never raw numbers.
		KeyMapping.Category category = KeyMapping.Category.register(id(MOD_ID));
		menuKey = KeyMappingHelper.registerKeyMapping(
				new KeyMapping("key.armstrong.menu", InputConstants.KEY_RSHIFT, category));
		KeyMapping zoomKey = KeyMappingHelper.registerKeyMapping(
				new KeyMapping("key.armstrong.zoom", InputConstants.KEY_C, category));

		modules = new ModuleManager(FabricLoader.getInstance().getConfigDir().resolve("armstrong.json"));
		KeyMapping snaplookKey = key("snaplook", InputConstants.KEY_V, category);
		KeyMapping replayKey = key("replay_record", InputConstants.UNKNOWN.getValue(), category);
		KeyMapping stopwatchKey = key("stopwatch", InputConstants.UNKNOWN.getValue(), category);
		KeyMapping[] autoTextKeys = {
			key("auto_text_1", InputConstants.UNKNOWN.getValue(), category),
			key("auto_text_2", InputConstants.UNKNOWN.getValue(), category),
			key("auto_text_3", InputConstants.UNKNOWN.getValue(), category),
		};

		// Registration order is the menu's order: HUD, movement, view, chat, client.
		modules.register(new StatusHud());
		EffectsHud effects = modules.register(new EffectsHud());
		modules.register(new ArmorHud());
		modules.register(new KeystrokesHud());
		modules.register(new InfoHuds.ItemsHud());
		InfoHuds.PvpInfo pvpInfo = modules.register(new InfoHuds.PvpInfo());
		modules.register(new InfoHuds.Stopwatch(stopwatchKey));
		modules.register(new InfoHuds.PackDisplay());
		Crosshair crosshair = modules.register(new Crosshair());
		HudTweaks hudTweaks = modules.register(new HudTweaks());
		modules.register(new ToggleSprint());
		modules.register(new ToggleSneak());
		zoom = modules.register(new Zoom(zoomKey));
		modules.register(new ViewModules.Snaplook(snaplookKey));
		modules.register(new ViewModules.Fullbright());
		modules.register(new ViewModules.TimeChanger());
		modules.register(new ViewModules.WeatherChanger());
		modules.register(new ViewModules.LowFire());
		modules.register(new ViewModules.Particles());
		modules.register(new ViewModules.Hitboxes());
		modules.register(new ViewModules.ChunkBorders());
		modules.register(new ChatModules.Chat());
		modules.register(new ChatModules.AutoText(autoTextKeys));
		ReplayModule replay = modules.register(new ReplayModule(replayKey));
		ui = modules.register(new Interface()); // last: it's settings, not a feature
		Theme.bind(ui);
		modules.load();
		Recent.init(FabricLoader.getInstance().getConfigDir().resolve("armstrong-recent.json"));
		// Remember the world or server for the home screen's continue entries.
		ClientPlayConnectionEvents.JOIN.register((handler, sender, mc) -> {
			Recent.joined(mc);
			replay.onJoin();
		});

		ClientTickEvents.END_CLIENT_TICK.register(mc -> {
			while (menuKey.consumeClick()) {
				if (mc.gui.screen() == null) {
					mc.setScreenAndShow(new ArmstrongScreen(modules, menuKey));
				}
			}
			modules.tick(mc);
		});
		HudElementRegistry.attachElementBefore(VanillaHudElements.CHAT, id("modules"),
				(g, delta) -> modules.renderHud(g, Minecraft.getInstance()));
		// The effects panel replaces the game's effect icons (a setting on the module).
		HudElementRegistry.replaceElement(VanillaHudElements.MOB_EFFECTS, vanilla -> (g, delta) -> {
			if (!effects.hidesVanilla()) {
				vanilla.extractRenderState(g, delta);
			}
		});
		// The custom crosshair, shown where the game would show its own (first person).
		HudElementRegistry.replaceElement(VanillaHudElements.CROSSHAIR, vanilla -> (g, delta) -> {
			Minecraft mc = Minecraft.getInstance();
			if (!crosshair.isEnabled()) {
				vanilla.extractRenderState(g, delta);
			} else if (mc.options.getCameraType().isFirstPerson()) {
				crosshair.draw(g);
			}
		});
		HudElementRegistry.replaceElement(VanillaHudElements.SCOREBOARD, hudTweaks::scoreboard);
		HudElementRegistry.replaceElement(VanillaHudElements.BOSS_BAR, hudTweaks::bossBar);
		HudElementRegistry.replaceElement(VanillaHudElements.TITLE_AND_SUBTITLE, hudTweaks::titles);
		// Hits you make, for combo and reach.
		AttackEntityCallback.EVENT.register((player, level, hand, entity, hit) -> {
			if (level.isClientSide() && pvpInfo.isEnabled()) {
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
