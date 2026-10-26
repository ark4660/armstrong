package com.ark4660.armstrong.compat;

import java.util.Locale;

import org.lwjgl.input.Keyboard;
import org.lwjgl.input.Mouse;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.gui.screen.Screen;
import net.minecraft.client.option.GameOptions;
import net.minecraft.client.option.KeyBinding;

/** The 1.8.9 versions of the modern builds' compat.Mc calls. */
public final class Mc {
	private Mc() {}

	/** The world field of view last used for rendering, after zoom (GameRendererMixin). */
	public static volatile float fov = 70f;

	/** The frame's partial tick, set before the HUD is drawn (for smooth markers). */
	public static volatile float partialTick;

	/** Where the camera is: its position and angles (yaw 0 = south, pitch + = down). */
	public record View(net.minecraft.util.math.Vec3d pos, float yaw, float pitch) {}

	/** The camera, from the view entity's eyes (third person's pull-back is ignored). */
	public static View view() {
		MinecraftClient mc = MinecraftClient.getInstance();
		net.minecraft.entity.Entity e = mc.getCameraEntity();
		float t = partialTick;
		double x = e.prevX + (e.x - e.prevX) * t;
		double y = e.prevY + (e.y - e.prevY) * t + e.getEyeHeight();
		double z = e.prevZ + (e.z - e.prevZ) * t;
		com.ark4660.armstrong.modules.Freelook f = com.ark4660.armstrong.ArmstrongClient.active(com.ark4660.armstrong.modules.Freelook.class);
		if (f != null && f.active()) {
			return new View(new net.minecraft.util.math.Vec3d(x, y, z), f.yaw(), f.pitch());
		}
		return new View(new net.minecraft.util.math.Vec3d(x, y, z),
				e.prevYaw + (e.yaw - e.prevYaw) * t, e.prevPitch + (e.pitch - e.prevPitch) * t);
	}

	/** The dimension's id, named like the modern ones (minecraft:the_nether). */
	public static String dimension(net.minecraft.world.World world) {
		return switch (world.dimension.getType()) {
			case -1 -> "minecraft:the_nether";
			case 1 -> "minecraft:the_end";
			case 0 -> "minecraft:overworld";
			default -> "dim:" + world.dimension.getType();
		};
	}

	/** Runs on the client thread (textures must be made there). */
	public static void onRenderThread(Runnable r) {
		MinecraftClient.getInstance().submit(r);
	}

	/** Registers a PNG as a texture under id; false if it can't be read. */
	public static boolean registerTexture(net.minecraft.util.Identifier id, byte[] png) {
		try {
			java.awt.image.BufferedImage image = javax.imageio.ImageIO.read(new java.io.ByteArrayInputStream(png));
			if (image == null) {
				return false;
			}
			MinecraftClient.getInstance().getTextureManager().loadTexture(id,
					new net.minecraft.client.texture.NativeImageBackedTexture(image));
			return true;
		} catch (java.io.IOException | RuntimeException e) {
			com.ark4660.armstrong.ArmstrongClient.LOG.warn("Couldn't load texture {}: {}", id, e.toString());
			return false;
		}
	}

	/** Whether an Alt key is held. */
	public static boolean altDown() {
		return Keyboard.isKeyDown(Keyboard.KEY_LMENU) || Keyboard.isKeyDown(Keyboard.KEY_RMENU);
	}

	/** The open screen, or null in-game. */
	public static Screen screen() {
		return MinecraftClient.getInstance().currentScreen;
	}

	/** Opens a screen (null closes it). */
	public static void open(Screen screen) {
		MinecraftClient.getInstance().setScreen(screen);
	}

	public static String versionName() {
		return "1.8.9";
	}

	/** Whether a key press in a screen is the key bound to a binding. */
	public static boolean isBound(KeyBinding binding, int key) {
		return binding.getCode() > 0 && binding.getCode() == key;
	}

	/**
	 * Whether a binding's key is held. Like 1.21.1, only one binding per key
	 * registers as held (zoom on C would lose to anything else on C), so the
	 * physical key is read too. Mouse buttons are negative codes (-100 = left).
	 */
	public static boolean held(KeyBinding binding) {
		if (binding.isPressed()) {
			return true;
		}
		int code = binding.getCode();
		if (code < 0) {
			return Mouse.isCreated() && Mouse.isButtonDown(code + 100);
		}
		return code > 0 && Keyboard.isCreated() && Keyboard.isKeyDown(code);
	}

	/** The key's name for the menu, like right_shift; "none" when unbound. */
	public static String keyName(KeyBinding binding) {
		if (binding == null) {
			return "-";
		}
		if (binding.getCode() == 0) {
			return "none";
		}
		return GameOptions.getFormattedNameForKeyCode(binding.getCode()).toLowerCase(Locale.ROOT).replace(' ', '_');
	}
}
