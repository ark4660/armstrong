package com.ark4660.armstrong.modules;

import java.util.ArrayList;
import java.util.List;

import com.ark4660.armstrong.module.HudModule;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.minecraft.client.Minecraft;
import net.minecraft.world.entity.EquipmentSlot;
import net.minecraft.world.item.ItemStack;

/** Armor durability as bars: helmet  ████████░░  82%. */
public final class ArmorHud extends HudModule.Text {
	private static final EquipmentSlot[] SLOTS = {EquipmentSlot.HEAD, EquipmentSlot.CHEST, EquipmentSlot.LEGS, EquipmentSlot.FEET};
	private static final String[] NAMES = {"helmet", "chest", "legs", "boots"};
	private static final int CELLS = 10;

	private final Setting.Bool hideEmpty = add(new Setting.Bool("hide_empty", "hide_empty_slots", false));
	private final Setting.Number warnAt = add(new Setting.Number("warn_at", "warn_below", 66, 5, 95, 1, "%"));

	public ArmorHud() {
		super("armor", "Armor", "Armor durability bars.", true, "Bottom right");
	}

	@Override
	protected List<Term.Line> lines(Minecraft mc) {
		if (mc.player == null) {
			return List.of();
		}
		List<Term.Line> out = new ArrayList<>();
		boolean any = false;
		for (int i = 0; i < SLOTS.length; i++) {
			ItemStack stack = mc.player.getItemBySlot(SLOTS[i]);
			Term.Line line = new Term.Line().add(Term.pad(NAMES[i], 8), Theme.TEXT);
			if (stack.isEmpty()) {
				if (hideEmpty.get()) {
					continue;
				}
				line.bar(CELLS, Theme.TRACK).add("    -", Theme.MUTED);
			} else if (!stack.isDamageableItem() || stack.getMaxDamage() <= 0) {
				line.bar(CELLS, Theme.accent()).add("    ∞", Theme.MUTED);
				any = true;
			} else {
				any = true;
				double frac = 1.0 - (double) stack.getDamageValue() / stack.getMaxDamage();
				int pct = (int) Math.round(frac * 100);
				int filled = Math.clamp(Math.round(frac * CELLS), 0, CELLS);
				int color = pct < 25 ? Theme.RED : pct < warnAt.get() ? Theme.AMBER : Theme.accent();
				line.bar(filled, color).bar(CELLS - filled, Theme.TRACK)
						.add(String.format("%5s", pct + "%"), Theme.TEXT);
			}
			out.add(line);
		}
		if (!any && hideEmpty.get()) {
			return List.of();
		}
		out.addFirst(new Term.Line().add("# armor", Theme.MUTED));
		return out;
	}
}
