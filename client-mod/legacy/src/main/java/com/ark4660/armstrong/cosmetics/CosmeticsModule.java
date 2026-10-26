package com.ark4660.armstrong.cosmetics;

import java.util.UUID;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.text.LiteralText;
import net.minecraft.text.Style;
import net.minecraft.text.Text;
import net.minecraft.util.Formatting;
import net.minecraft.util.Identifier;

/**
 * Shows Armstrong capes and name badges (yours and other players'). 1.8.9
 * text has only the 16 chat colours, so a badge takes the nearest one.
 */
public final class CosmeticsModule extends Module {
	private final Setting.Bool capes = add(new Setting.Bool("capes", "show_capes", true));
	private final Setting.Bool badges = add(new Setting.Bool("badges", "show_badges", true));

	/** The chat colours and their RGB, for the nearest match. */
	private static final Formatting[] COLORS = {
		Formatting.BLACK, Formatting.DARK_BLUE, Formatting.DARK_GREEN, Formatting.DARK_AQUA, Formatting.DARK_RED,
		Formatting.DARK_PURPLE, Formatting.GOLD, Formatting.GRAY, Formatting.DARK_GRAY, Formatting.BLUE,
		Formatting.GREEN, Formatting.AQUA, Formatting.RED, Formatting.LIGHT_PURPLE, Formatting.YELLOW, Formatting.WHITE,
	};
	private static final int[] RGB = {
		0x000000, 0x0000AA, 0x00AA00, 0x00AAAA, 0xAA0000, 0xAA00AA, 0xFFAA00, 0xAAAAAA,
		0x555555, 0x5555FF, 0x55FF55, 0x55FFFF, 0xFF5555, 0xFF55FF, 0xFFFF55, 0xFFFFFF,
	};

	public CosmeticsModule() {
		super("cosmetics", "Cosmetics", "Armstrong capes and name badges, yours and other players'.", Category.CLIENT, true);
	}

	/** The cape to draw on a player, or null (for the cape mixin). */
	public static Identifier cape(UUID player) {
		CosmeticsModule m = ArmstrongClient.active(CosmeticsModule.class);
		return m != null && m.capes.get() ? Cosmetics.cape(player) : null;
	}

	private static Cosmetics.Badge badge(UUID player) {
		CosmeticsModule m = ArmstrongClient.active(CosmeticsModule.class);
		return m != null && m.badges.get() ? Cosmetics.badge(player) : null;
	}

	/** A player's name with their badge in front, if they have one. */
	public static Text withBadge(UUID player, Text name) {
		Cosmetics.Badge b = badge(player);
		if (b == null) {
			return name;
		}
		Text out = new LiteralText(b.symbol() + " ").setStyle(new Style().setFormatting(nearest(b.rgb())));
		return out.append(name);
	}

	/** The same for the tab list, which takes formatted strings. */
	public static String withBadge(UUID player, String name) {
		Cosmetics.Badge b = badge(player);
		return b == null ? name : nearest(b.rgb()) + b.symbol() + " " + Formatting.RESET + name;
	}

	private static Formatting nearest(int rgb) {
		int best = 0;
		long bestDist = Long.MAX_VALUE;
		for (int i = 0; i < RGB.length; i++) {
			long dr = ((rgb >> 16) & 0xFF) - ((RGB[i] >> 16) & 0xFF);
			long dg = ((rgb >> 8) & 0xFF) - ((RGB[i] >> 8) & 0xFF);
			long db = (rgb & 0xFF) - (RGB[i] & 0xFF);
			long d = dr * dr + dg * dg + db * db;
			if (d < bestDist) {
				bestDist = d;
				best = i;
			}
		}
		return COLORS[best];
	}
}
