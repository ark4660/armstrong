package com.ark4660.armstrong.bench;

import java.util.ArrayDeque;
import java.util.LinkedHashMap;
import java.util.Map;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.gui.ArmstrongScreen;
import com.ark4660.armstrong.module.Module;

import net.minecraft.client.Minecraft;
import net.minecraft.client.player.LocalPlayer;

/**
 * Benchmark scenes for `armstrong bench` (enabled by -Darmstrong.bench.scene).
 * From the title screen this creates a fresh world with a fixed seed; once
 * the player is in, it holds the camera on a fixed path, waits out the
 * warm-up, records every frame for the measured window, writes the result
 * and quits the game.
 *
 * Scenes: static (stand still), fly (fly straight on, loading new chunks),
 * spin (turn on the spot), entities (a crowd of mobs in view), menu (the
 * Armstrong menu open over the world).
 */
public final class Bench {
	private static final double START_X = 0.5, START_Y = 120, START_Z = 0.5;
	/** Blocks per tick for fly (20 blocks a second, faster than creative flight). */
	private static final double FLY_SPEED = 1.0;
	private static final float SPIN_SPEED = 4f, PITCH = 15f;
	private static final int CROWD = 12; // a CROWD x CROWD grid of mobs
	private static final long SEED = Long.getLong("armstrong.bench.seed", 4660);
	/** Nothing changes but the scene: no day cycle, weather, mob spawns or chat feedback. */
	//? if >=26 {
	private static final String[] WORLD_COMMANDS = {"gamerule send_command_feedback false", "gamerule log_admin_commands false", "gamerule advance_time false",
			"gamerule advance_weather false", "gamerule spawn_mobs false", "time set 6000", "weather clear"};
	//?} else {
	/*private static final String[] WORLD_COMMANDS = {"gamerule sendCommandFeedback false", "gamerule logAdminCommands false", "gamerule doDaylightCycle false",
			"gamerule doWeatherCycle false", "gamerule doMobSpawning false", "time set 6000", "weather clear"};
	*///?}

	private static BenchRecorder rec;
	private static final ArrayDeque<String> commands = new ArrayDeque<>();
	private static int ticksInWorld, waitTicks;
	private static boolean creating;

	private Bench() {}

	public static boolean enabled() {
		return rec != null;
	}

	/** Called from onInitializeClient when benchmarking was asked for. */
	public static void start() {
		rec = new BenchRecorder();
		if (Boolean.getBoolean("armstrong.bench.noModules")) {
			// The baseline: the mod only measures, every feature is off.
			for (Module m : ArmstrongClient.modules().all()) {
				if (m.canDisable()) {
					m.setEnabled(false);
				}
			}
		}
		ArmstrongClient.LOG.info("Bench: scene {}, warm-up {}s, measuring {}s", rec.scene, rec.warmupNs / 1e9, rec.durationNs / 1e9);
	}

	/** End of every client tick. */
	public static void tick(Minecraft mc) {
		if (!Mc.loading()) {
			rec.markReady();
		}
		LocalPlayer p = mc.player;
		if (!rec.inWorld() && ++waitTicks % 100 == 0) {
			// A run that never starts is usually stuck on a screen: name it.
			ArmstrongClient.LOG.info("Bench: waiting for the world, screen {}", Mc.screen() == null ? "none" : Mc.screen().getClass().getName());
		}
		if (p == null || mc.level == null) {
			if (!creating && !Mc.loading() && mc.level == null && Mc.screen() != null) {
				creating = true;
				createWorld(mc);
			}
			return;
		}
		if (!rec.inWorld()) {
			if (Mc.screen() != null) {
				return; // still on the loading screen
			}
			setUp(mc, p);
			rec.markInWorld();
			ArmstrongClient.LOG.info("Bench: in world");
		}
		ticksInWorld++;
		for (int i = 0; i < 8 && !commands.isEmpty(); i++) {
			p.connection.sendCommand(commands.poll());
		}
		p.getAbilities().flying = true;
		p.setDeltaMovement(0, 0, 0);
		switch (rec.scene) {
			case "fly" -> place(p, START_X, START_Y, START_Z + ticksInWorld * FLY_SPEED, 0f, PITCH);
			case "spin" -> place(p, START_X, START_Y, START_Z, ticksInWorld * SPIN_SPEED, PITCH);
			default -> place(p, START_X, START_Y, START_Z, 0f, PITCH);
		}
	}

	/** A creative, peaceful world with cheats, made the way the Create World screen makes one. */
	private static void createWorld(Minecraft mc) {
		String name = System.getProperty("armstrong.bench.world", "armstrong-bench");
		//? if >=26 {
		net.minecraft.world.level.LevelSettings settings = new net.minecraft.world.level.LevelSettings(name,
				net.minecraft.world.level.GameType.CREATIVE,
				new net.minecraft.world.level.LevelSettings.DifficultySettings(net.minecraft.world.Difficulty.PEACEFUL, false, false),
				true, net.minecraft.world.level.WorldDataConfiguration.DEFAULT);
		//?} else {
		/*net.minecraft.world.level.LevelSettings settings = new net.minecraft.world.level.LevelSettings(name,
				net.minecraft.world.level.GameType.CREATIVE, false, net.minecraft.world.Difficulty.PEACEFUL, true,
				new net.minecraft.world.level.GameRules(), net.minecraft.world.level.WorldDataConfiguration.DEFAULT);
		*///?}
		ArmstrongClient.LOG.info("Bench: creating world {} (seed {})", name, SEED);
		mc.createWorldOpenFlows().createFreshLevel(name, settings, new net.minecraft.world.level.levelgen.WorldOptions(SEED, true, false),
				net.minecraft.world.level.levelgen.presets.WorldPresets::createNormalWorldDimensions, Mc.screen());
	}

	private static void setUp(Minecraft mc, LocalPlayer p) {
		commands.addAll(java.util.List.of(WORLD_COMMANDS));
		//? if >=26 {
		p.snapTo(START_X, START_Y, START_Z, 0f, PITCH);
		//?} else {
		/*p.moveTo(START_X, START_Y, START_Z, 0f, PITCH);
		*///?}
		if (rec.scene.equals("entities")) {
			// Floating villagers 6 to 30 blocks ahead, all in view.
			for (int x = 0; x < CROWD; x++) {
				for (int z = 0; z < CROWD; z++) {
					commands.add(String.format(java.util.Locale.ROOT,
							"summon minecraft:villager %.1f %.1f %.1f {NoAI:1b,NoGravity:1b,Silent:1b,PersistenceRequired:1b}",
							START_X + (x - CROWD / 2) * 2.0, START_Y - 4, START_Z + 6 + z * 2.0));
				}
			}
		}
		if (rec.scene.equals("menu")) {
			Mc.open(new ArmstrongScreen(ArmstrongClient.modules(), ArmstrongClient.menuKey()));
		}
	}

	/** Moves the camera; the frames in between interpolate from the last tick. */
	private static void place(LocalPlayer p, double x, double y, double z, float yaw, float pitch) {
		p.setPos(x, y, z);
		p.setYRot(yaw);
		p.setXRot(pitch);
	}

	/** Every rendered frame (from the HUD pass, which always runs in the world). */
	public static void frame() {
		if (!rec.frame(Minecraft.getInstance().isWindowActive())) {
			return;
		}
		Minecraft mc = Minecraft.getInstance();
		Map<String, Object> game = new LinkedHashMap<>();
		game.put("minecraft", Mc.versionName());
		//? if >=26 {
		com.mojang.renderpearl.api.device.DeviceInfo gpu = com.mojang.blaze3d.systems.RenderSystem.getDevice().getDeviceInfo();
		game.put("renderer", gpu.name());
		game.put("backend", gpu.backendName());
		game.put("driver", gpu.driverInfo());
		//?} else {
		/*game.put("renderer", com.mojang.blaze3d.platform.GlUtil.getRenderer());
		game.put("backend", "OpenGL");
		game.put("driver", com.mojang.blaze3d.platform.GlUtil.getOpenGLVersion());
		*///?}
		game.put("window", mc.getWindow().getWidth() + "x" + mc.getWindow().getHeight());
		game.put("renderDistance", mc.options.renderDistance().get());
		game.put("entities", mc.level == null ? 0 : mc.level.getEntityCount());
		try {
			rec.finish(game);
			ArmstrongClient.LOG.info("Bench: done");
		} catch (Exception e) {
			ArmstrongClient.LOG.error("Bench: couldn't write the result", e);
		}
		mc.stop();
	}
}
