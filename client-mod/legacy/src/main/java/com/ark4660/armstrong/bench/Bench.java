package com.ark4660.armstrong.bench;

import java.util.ArrayDeque;
import java.util.LinkedHashMap;
import java.util.Map;

import org.lwjgl.opengl.GL11;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.gui.ArmstrongScreen;
import com.ark4660.armstrong.module.Module;

import net.minecraft.client.MinecraftClient;
import net.minecraft.entity.player.ClientPlayerEntity;
import net.minecraft.world.level.LevelGeneratorType;
import net.minecraft.world.level.LevelInfo;

/**
 * Benchmark scenes for `armstrong bench` (1.8.9 port of ../src's Bench; see
 * there). Commands go through chat, and mobs take 1.8 entity names.
 */
public final class Bench {
	private static final double START_X = 0.5, START_Y = 120, START_Z = 0.5;
	private static final double FLY_SPEED = 1.0;
	private static final float SPIN_SPEED = 4f, PITCH = 15f;
	private static final int CROWD = 12;
	private static final long SEED = Long.getLong("armstrong.bench.seed", 4660);
	private static final String[] WORLD_COMMANDS = {"gamerule sendCommandFeedback false", "gamerule logAdminCommands false", "gamerule doDaylightCycle false",
			"gamerule doMobSpawning false", "time set 6000", "weather clear 1000000"};

	private static BenchRecorder rec;
	private static final ArrayDeque<String> commands = new ArrayDeque<>();
	private static int ticksInWorld;
	private static boolean joining;

	private Bench() {}

	public static boolean enabled() {
		return rec != null;
	}

	public static void start() {
		rec = new BenchRecorder();
		if (Boolean.getBoolean("armstrong.bench.noModules")) {
			for (Module m : ArmstrongClient.modules().all()) {
				if (m.canDisable()) {
					m.setEnabled(false);
				}
			}
		}
		ArmstrongClient.LOG.info("Bench: scene {}, warm-up {}s, measuring {}s", rec.scene, rec.warmupNs / 1e9, rec.durationNs / 1e9);
	}

	public static void tick(MinecraftClient mc) {
		rec.markReady(); // 1.8.9 loads resources before the first tick
		ClientPlayerEntity p = mc.player;
		if (p == null || mc.world == null) {
			// A fresh creative world with cheats, from the title screen.
			String world = System.getProperty("armstrong.bench.world", "armstrong-bench");
			if (!joining && mc.world == null && Mc.screen() != null) {
				joining = true;
				ArmstrongClient.LOG.info("Bench: creating world {} (seed {})", world, SEED);
				mc.startIntegratedServer(world, world, new LevelInfo(SEED, LevelInfo.GameMode.CREATIVE, true, false, LevelGeneratorType.DEFAULT).enableCommands());
			}
			return;
		}
		if (!rec.inWorld()) {
			if (Mc.screen() != null) {
				return;
			}
			setUp(p);
			rec.markInWorld();
			ArmstrongClient.LOG.info("Bench: in world");
		}
		ticksInWorld++;
		for (int i = 0; i < 8 && !commands.isEmpty(); i++) {
			p.sendChatMessage("/" + commands.poll());
		}
		p.abilities.flying = true;
		p.velocityX = p.velocityY = p.velocityZ = 0;
		switch (rec.scene) {
			case "fly" -> place(p, START_X, START_Y, START_Z + ticksInWorld * FLY_SPEED, 0f, PITCH);
			case "spin" -> place(p, START_X, START_Y, START_Z, ticksInWorld * SPIN_SPEED, PITCH);
			default -> place(p, START_X, START_Y, START_Z, 0f, PITCH);
		}
	}

	private static void setUp(ClientPlayerEntity p) {
		commands.addAll(java.util.List.of(WORLD_COMMANDS));
		p.refreshPositionAndAngles(START_X, START_Y, START_Z, 0f, PITCH);
		if (rec.scene.equals("entities")) {
			for (int x = 0; x < CROWD; x++) {
				for (int z = 0; z < CROWD; z++) {
					commands.add(String.format(java.util.Locale.ROOT,
							"summon Villager %.1f %.1f %.1f {NoAI:1,Silent:1,PersistenceRequired:1}",
							START_X + (x - CROWD / 2) * 2.0, START_Y - 4, START_Z + 6 + z * 2.0));
				}
			}
		}
		if (rec.scene.equals("menu")) {
			Mc.open(new ArmstrongScreen(ArmstrongClient.modules(), ArmstrongClient.menuKey()));
		}
	}

	private static void place(ClientPlayerEntity p, double x, double y, double z, float yaw, float pitch) {
		p.updatePosition(x, y, z);
		p.yaw = yaw;
		p.pitch = pitch;
	}

	public static void frame() {
		if (!rec.frame(org.lwjgl.opengl.Display.isActive())) {
			return;
		}
		MinecraftClient mc = MinecraftClient.getInstance();
		Map<String, Object> game = new LinkedHashMap<>();
		game.put("minecraft", Mc.versionName());
		game.put("renderer", GL11.glGetString(GL11.GL_RENDERER));
		game.put("backend", "OpenGL");
		game.put("driver", GL11.glGetString(GL11.GL_VERSION));
		game.put("window", mc.width + "x" + mc.height);
		game.put("renderDistance", mc.options.viewDistance);
		game.put("entities", mc.world == null ? 0 : mc.world.loadedEntities.size());
		try {
			rec.finish(game);
			ArmstrongClient.LOG.info("Bench: done");
		} catch (Exception e) {
			ArmstrongClient.LOG.error("Bench: couldn't write the result", e);
		}
		mc.scheduleStop();
	}
}
