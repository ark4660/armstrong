package com.ark4660.armstrong.replay;

import java.lang.reflect.Field;
import java.lang.reflect.Method;

import com.ark4660.armstrong.ArmstrongClient;

import net.fabricmc.loader.api.FabricLoader;
import net.minecraft.client.gui.screens.Screen;

/**
 * Bridge to the Flashback replay mod (an optional client add-on). Calls go
 * through reflection so Armstrong works the same without it; every method is
 * a safe no-op when Flashback isn't installed or its API changed.
 */
public final class Flashback {
	private static final String MAIN = "com.moulberry.flashback.Flashback";
	private static final String SELECT = "com.moulberry.flashback.screen.select_replay.SelectReplayScreen";

	private Flashback() {}

	public static boolean installed() {
		return FabricLoader.getInstance().isModLoaded("flashback");
	}

	/** Flashback's replay browser, or null if it can't be opened. */
	public static Screen replayScreen(Screen parent) {
		if (!installed()) {
			return null;
		}
		try {
			return (Screen) Class.forName(SELECT).getConstructor(Screen.class).newInstance(parent);
		} catch (ReflectiveOperationException | ClassCastException e) {
			ArmstrongClient.LOG.warn("Couldn't open Flashback's replay list: {}", e.toString());
			return null;
		}
	}

	// Looked up once: recording() and inReplay() run every frame.
	private static volatile boolean resolved;
	private static Field recorder;
	private static Method isInReplay;

	private static void resolve() {
		if (resolved) {
			return;
		}
		if (installed()) {
			try {
				Class<?> main = Class.forName(MAIN);
				recorder = main.getField("RECORDER");
				isInReplay = main.getMethod("isInReplay");
			} catch (ReflectiveOperationException e) {
				ArmstrongClient.LOG.warn("Flashback's API changed; replay features are off: {}", e.toString());
			}
		}
		resolved = true;
	}

	public static boolean recording() {
		resolve();
		try {
			return recorder != null && recorder.get(null) != null;
		} catch (ReflectiveOperationException e) {
			return false;
		}
	}

	/** Whether a replay is being watched (Flashback's editor is open). */
	public static boolean inReplay() {
		resolve();
		try {
			return isInReplay != null && isInReplay.invoke(null) instanceof Boolean b && b;
		} catch (ReflectiveOperationException e) {
			return false;
		}
	}

	public static void start() {
		call("startRecordingReplay");
	}

	public static void finish() {
		call("finishRecordingReplay");
	}

	private static Object call(String method) {
		if (!installed()) {
			return null;
		}
		try {
			Method m = Class.forName(MAIN).getMethod(method);
			return m.invoke(null);
		} catch (ReflectiveOperationException e) {
			ArmstrongClient.LOG.warn("Flashback {} failed: {}", method, e.toString());
			return null;
		}
	}
}
