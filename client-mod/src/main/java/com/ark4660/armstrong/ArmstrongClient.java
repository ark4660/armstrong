package com.ark4660.armstrong;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import com.ark4660.armstrong.gui.ArmstrongScreen;
import com.ark4660.armstrong.home.Recent;
import com.ark4660.armstrong.module.ModuleManager;
import com.ark4660.armstrong.modules.ArmorHud;
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

	@Override
	public void onInitializeClient() {
		// 26.x key codes are SDL scancodes: use InputConstants names, never raw numbers.
		KeyMapping.Category category = KeyMapping.Category.register(id(MOD_ID));
		menuKey = KeyMappingHelper.registerKeyMapping(
				new KeyMapping("key.armstrong.menu", InputConstants.KEY_RSHIFT, category));
		KeyMapping zoomKey = KeyMappingHelper.registerKeyMapping(
				new KeyMapping("key.armstrong.zoom", InputConstants.KEY_C, category));

		modules = new ModuleManager(FabricLoader.getInstance().getConfigDir().resolve("armstrong.json"));
		modules.register(new StatusHud());
		EffectsHud effects = modules.register(new EffectsHud());
		modules.register(new ArmorHud());
		modules.register(new KeystrokesHud());
		modules.register(new ToggleSprint());
		zoom = modules.register(new Zoom(zoomKey));
		ui = modules.register(new Interface()); // last: it's settings, not a feature
		Theme.bind(ui);
		modules.load();
		Recent.init(FabricLoader.getInstance().getConfigDir().resolve("armstrong-recent.json"));
		// Remember the world or server for the home screen's continue entries.
		ClientPlayConnectionEvents.JOIN.register((handler, sender, mc) -> Recent.joined(mc));

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
		ClientLifecycleEvents.CLIENT_STOPPING.register(mc -> modules.save());

		String version = FabricLoader.getInstance().getModContainer(MOD_ID)
				.map(c -> c.getMetadata().getVersion().getFriendlyString()).orElse("?");
		LOG.info("Armstrong {} ready with {} modules", version, modules.all().size());
	}
}
