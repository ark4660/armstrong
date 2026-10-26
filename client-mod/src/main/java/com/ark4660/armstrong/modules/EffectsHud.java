package com.ark4660.armstrong.modules;

import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

import com.ark4660.armstrong.module.HudModule;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.minecraft.client.Minecraft;
import net.minecraft.network.chat.Component;
import net.minecraft.world.effect.MobEffectInstance;

/** Active potion effects as a list with timers: > speed II  01:24. */
public final class EffectsHud extends HudModule.Text {
	private static final String[] ROMAN = {"", " II", " III", " IV", " V", " VI", " VII", " VIII", " IX", " X"};

	private final Setting.Bool hideVanilla = add(new Setting.Bool("hide_vanilla", "hide_vanilla_icons", true));

	public EffectsHud() {
		super("effects", "Effects", "Active effects with time left.", true, "Top right");
	}

	/** Whether the game's own effect icons should be hidden. */
	public boolean hidesVanilla() {
		return isEnabled() && hideVanilla.get();
	}

	@Override
	protected List<Term.Line> lines(Minecraft mc) {
		if (mc.player == null || mc.player.getActiveEffects().isEmpty()) {
			return List.of();
		}
		List<String> names = new ArrayList<>();
		List<String> times = new ArrayList<>();
		int width = 0;
		for (MobEffectInstance e : mc.player.getActiveEffects()) {
			String name = Component.translatable(e.getDescriptionId()).getString().toLowerCase(Locale.ROOT).replace(' ', '_');
			int amp = e.getAmplifier();
			name += amp >= 0 && amp < ROMAN.length ? ROMAN[amp] : " " + (amp + 1);
			names.add(name);
			times.add(e.isInfiniteDuration() ? "∞" : clock(e.getDuration() / 20));
			width = Math.max(width, name.length());
		}
		int accent = Theme.accent();
		List<Term.Line> out = new ArrayList<>();
		out.add(new Term.Line().add("# effects", Theme.MUTED));
		for (int i = 0; i < names.size(); i++) {
			out.add(new Term.Line().add("> ", accent).add(Term.pad(names.get(i), width + 2), Theme.TEXT).add(times.get(i), Theme.AMBER));
		}
		return out;
	}

	private static String clock(int seconds) {
		if (seconds >= 3600) {
			return String.format("%d:%02d:%02d", seconds / 3600, seconds / 60 % 60, seconds % 60);
		}
		return String.format("%02d:%02d", seconds / 60, seconds % 60);
	}
}
