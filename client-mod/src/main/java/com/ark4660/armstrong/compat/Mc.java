package com.ark4660.armstrong.compat;

import com.mojang.blaze3d.platform.InputConstants;

import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;
//? if >=26 {
import net.fabricmc.fabric.api.client.keymapping.v1.KeyMappingHelper;
//?} else {
/*import net.fabricmc.fabric.api.client.keybinding.v1.KeyBindingHelper;
*///?}
import net.minecraft.client.gui.screens.Screen;

/**
 * Calls that differ between Minecraft versions, so the rest of the mod can
 * stay version-neutral. Each method is one //? if block (see settings.gradle).
 */
public final class Mc {
	private Mc() {}

	/** The world field of view last used for rendering, after zoom (CameraMixin). */
	public static volatile float fov = 70f;

	/** Where the camera is: its position and angles (yaw 0 = south, pitch + = down). */
	public record View(net.minecraft.world.phys.Vec3 pos, float yaw, float pitch) {}

	public static View view() {
		Minecraft mc = Minecraft.getInstance();
		//? if >=26 {
		net.minecraft.client.Camera c = mc.gameRenderer.mainCamera();
		return new View(c.position(), c.yRot(), c.xRot());
		//?} else {
		/*net.minecraft.client.Camera c = mc.gameRenderer.getMainCamera();
		return new View(c.getPosition(), c.getYRot(), c.getXRot());
		*///?}
	}

	/** The dimension's id, like minecraft:the_nether. */
	public static String dimension(net.minecraft.world.level.Level level) {
		//? if >=26 {
		return level.dimension().identifier().toString();
		//?} else {
		/*return level.dimension().location().toString();
		*///?}
	}

	/** The open screen, or null in-game. */
	public static Screen screen() {
		//? if >=26 {
		return Minecraft.getInstance().gui.screen();
		//?} else {
		/*return Minecraft.getInstance().screen;
		*///?}
	}

	/** Opens a screen (null closes it). */
	public static void open(Screen screen) {
		//? if >=26 {
		Minecraft.getInstance().setScreenAndShow(screen);
		//?} else {
		/*Minecraft.getInstance().setScreen(screen);
		*///?}
	}

	/** The Minecraft version's display name, like 1.21.1. */
	public static String versionName() {
		//? if >=26 {
		return net.minecraft.SharedConstants.getCurrentVersion().name();
		//?} else {
		/*return net.minecraft.SharedConstants.getCurrentVersion().getName();
		*///?}
	}

	/** Whether an Alt key is held. */
	public static boolean altDown() {
		//? if >=26 {
		return Minecraft.getInstance().hasAltDown();
		//?} else {
		/*return Screen.hasAltDown();
		*///?}
	}

	/** Whether a keyboard key press (from CompatScreen.onKey) is the key bound to a mapping. */
	public static boolean isBound(KeyMapping mapping, int key) {
		//? if >=26 {
		InputConstants.Key bound = KeyMappingHelper.getBoundKeyOf(mapping);
		//?} else {
		/*InputConstants.Key bound = KeyBindingHelper.getBoundKeyOf(mapping);
		*///?}
		return bound.getType() != InputConstants.Type.MOUSE && bound.getValue() == key;
	}

	/**
	 * Whether a key mapping's key is held. Before 26.x only one mapping per
	 * key registers as held (zoom on C shares C with Save Hotbar Activator),
	 * so the physical key is read too.
	 */
	public static boolean held(KeyMapping mapping) {
		if (mapping.isDown()) {
			return true;
		}
		//? if >=26 {
		return false;
		//?} else {
		/*InputConstants.Key bound = KeyBindingHelper.getBoundKeyOf(mapping);
		long window = Minecraft.getInstance().getWindow().getWindow();
		if (bound.getType() == InputConstants.Type.MOUSE) {
			return org.lwjgl.glfw.GLFW.glfwGetMouseButton(window, bound.getValue()) == org.lwjgl.glfw.GLFW.GLFW_PRESS;
		}
		return bound.getValue() != InputConstants.UNKNOWN.getValue() && InputConstants.isKeyDown(window, bound.getValue());
		*///?}
	}
}
