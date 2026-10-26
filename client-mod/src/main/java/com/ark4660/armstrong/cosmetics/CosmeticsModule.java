package com.ark4660.armstrong.cosmetics;

import java.util.UUID;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.network.chat.Component;
import net.minecraft.network.chat.MutableComponent;
import net.minecraft.resources.Identifier;

/** Shows Armstrong capes and name badges (yours and other players'). */
public final class CosmeticsModule extends Module {
	private final Setting.Bool capes = add(new Setting.Bool("capes", "show_capes", true));
	private final Setting.Bool badges = add(new Setting.Bool("badges", "show_badges", true));

	public CosmeticsModule() {
		super("cosmetics", "Cosmetics", "Armstrong capes and name badges, yours and other players'.", Category.CLIENT, true);
	}

	/** The cape to draw on a player, or null (for the skin mixin). */
	public static Identifier cape(UUID player) {
		CosmeticsModule m = ArmstrongClient.active(CosmeticsModule.class);
		return m != null && m.capes.get() ? Cosmetics.cape(player) : null;
	}

	/** A player's name with their badge in front, if they have one. */
	public static Component withBadge(UUID player, Component name) {
		CosmeticsModule m = ArmstrongClient.active(CosmeticsModule.class);
		Cosmetics.Badge b = m != null && m.badges.get() ? Cosmetics.badge(player) : null;
		if (b == null) {
			return name;
		}
		MutableComponent out = Component.literal(b.symbol() + " ").withStyle(s -> s.withColor(b.rgb()));
		return out.append(name);
	}
}
