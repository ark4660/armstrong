package com.ark4660.armstrong.ui;

import com.ark4660.armstrong.modules.Interface;

/**
 * Armstrong's terminal look: near-black panels, a phosphor accent, amber for
 * selection and warnings. Colors are ARGB.
 */
public final class Theme {
	public static final int WINDOW = 0xFF080C0A;
	public static final int PANEL = 0xD1080C0A; // HUD panels, ~82% opaque
	public static final int BORDER = 0xFF24342A;
	public static final int RULE = 0xFF1A261F;
	public static final int TEXT = 0xFFC8D3CC;
	public static final int BRIGHT = 0xFFFFFFFF;
	public static final int MUTED = 0xFF8A9A8F;
	public static final int AMBER = 0xFFFFB454;
	public static final int CYAN = 0xFF58C4FF;
	public static final int RED = 0xFFFF6B6B;
	public static final int TRACK = 0xFF3A4A3F; // empty part of bars
	public static final int ON_ACCENT = 0xFF07100A; // text on an accent fill

	private static Interface settings;
	// The accent is read many times a frame: remember the last choice's color.
	private static String accentName;
	private static int accentColor = 0xFF5EF38C;

	private Theme() {}

	public static void bind(Interface module) {
		settings = module;
	}

	public static int accent() {
		if (settings == null) {
			return 0xFF5EF38C;
		}
		String name = settings.accent();
		if (!name.equals(accentName)) {
			accentName = name;
			accentColor = switch (name) {
				case "Amber" -> AMBER;
				case "Cyan" -> CYAN;
				default -> 0xFF5EF38C;
			};
		}
		return accentColor;
	}

	/** The accent at low opacity, for selected rows. */
	public static int accentDim() {
		return (accent() & 0x00FFFFFF) | 0x26000000;
	}

	public static boolean monospace() {
		return settings == null || settings.monospace();
	}
}
