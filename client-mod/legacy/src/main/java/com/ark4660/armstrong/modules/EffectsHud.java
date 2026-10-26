package com.ark4660.armstrong.modules;

import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

import com.ark4660.armstrong.module.HudModule;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.resource.language.I18n;
import net.minecraft.entity.effect.StatusEffectInstance;

/** Active potion effects as a list with timers: > speed II  01:24. */
public final class EffectsHud extends HudModule.Text {
	private static final String[] ROMAN = {"", " II", " III", " IV", " V", " VI", " VII", " VIII", " IX", " X"};

	public EffectsHud() {
		super("effects", "Effects", "Active effects with time left.", true, "Top right");
	}

	@Override
	protected List<Term.Line> lines(MinecraftClient mc) {
		if (mc.player == null || mc.player.getStatusEffectInstances().isEmpty()) {
			return List.of();
		}
		List<String> names = new ArrayList<>();
		List<String> times = new ArrayList<>();
		int width = 0;
		for (StatusEffectInstance e : mc.player.getStatusEffectInstances()) {
			String name = I18n.translate(e.getTranslationKey()).toLowerCase(Locale.ROOT).replace(' ', '_');
			int amp = e.getAmplifier();
			name += amp >= 0 && amp < ROMAN.length ? ROMAN[amp] : " " + (amp + 1);
			names.add(name);
			times.add(clock(e.getDuration() / 20));
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
