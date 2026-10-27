package com.ark4660.armstrong.modules;

import java.util.ArrayList;
import java.util.List;

import com.ark4660.armstrong.module.HudModule;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;
import net.minecraft.server.packs.repository.Pack;
import net.minecraft.world.entity.Entity;
import net.minecraft.world.entity.player.Inventory;
import net.minecraft.world.item.ItemStack;
import net.minecraft.world.item.Items;
import net.minecraft.world.phys.AABB;
import net.minecraft.world.phys.Vec3;

/** Small text HUD panels. */
public final class InfoHuds {
	private InfoHuds() {}

	/** Press the key to start, again to stop, a third time to reset. */
	public static final class Stopwatch extends HudModule.Text {
		private final KeyMapping key;
		private long started, elapsed;
		private boolean running;

		public Stopwatch(KeyMapping key) {
			super("stopwatch", "Stopwatch", "Press the key: start, stop, reset.", false, "Top left");
			this.key = key;
		}

		@Override
		public KeyMapping key() {
			return key;
		}

		@Override
		public void tick(Minecraft mc) {
			while (key.consumeClick()) {
				if (running) {
					elapsed += System.nanoTime() - started;
					running = false;
				} else if (elapsed > 0) {
					elapsed = 0;
				} else {
					started = System.nanoTime();
					running = true;
				}
			}
		}

		@Override
		protected boolean live() {
			return running; // hundredths of a second
		}

		@Override
		protected List<Term.Line> lines(Minecraft mc) {
			long ns = elapsed + (running ? System.nanoTime() - started : 0);
			long cs = ns / 10_000_000;
			String time = String.format("%02d:%02d.%02d", cs / 6000, cs / 100 % 60, cs % 100);
			return List.of(new Term.Line().add("stopwatch ", Theme.MUTED).add(time, running ? Theme.accent() : Theme.AMBER));
		}
	}

	/** Your current combo and the distance of your last hit. */
	public static final class PvpInfo extends HudModule.Text {
		private final Setting.Bool combo = add(new Setting.Bool("combo", "combo", true));
		private final Setting.Bool reach = add(new Setting.Bool("reach", "reach", true));
		private int hits;
		private int lastTarget = -1;
		private long lastHit;
		private double lastReach = -1;
		private int lastHurt;

		public PvpInfo() {
			super("pvp_info", "PvP Info", "Combo counter and reach of your last hit.", false, "Top left");
		}

		/** Called when you hit an entity. */
		public void onAttack(Minecraft mc, Entity target) {
			if (mc.player == null) {
				return;
			}
			hits = target.getId() == lastTarget && System.currentTimeMillis() - lastHit < 3000 ? hits + 1 : 1;
			lastTarget = target.getId();
			lastHit = System.currentTimeMillis();
			Vec3 eye = mc.player.getEyePosition();
			AABB box = target.getBoundingBox();
			double dx = Math.max(Math.max(box.minX - eye.x, 0), eye.x - box.maxX);
			double dy = Math.max(Math.max(box.minY - eye.y, 0), eye.y - box.maxY);
			double dz = Math.max(Math.max(box.minZ - eye.z, 0), eye.z - box.maxZ);
			lastReach = Math.sqrt(dx * dx + dy * dy + dz * dz);
		}

		@Override
		public void tick(Minecraft mc) {
			if (mc.player == null) {
				return;
			}
			// Getting hit, or 3 s without a hit, ends the combo.
			int hurt = mc.player.hurtTime;
			if ((hurt > lastHurt) || System.currentTimeMillis() - lastHit > 3000) {
				hits = 0;
			}
			lastHurt = hurt;
		}

		@Override
		protected List<Term.Line> lines(Minecraft mc) {
			List<Term.Line> out = new ArrayList<>();
			if (combo.get()) {
				out.add(new Term.Line().add(Term.pad("combo", 6), Theme.MUTED).add(String.valueOf(hits), hits > 1 ? Theme.accent() : Theme.TEXT));
			}
			if (reach.get()) {
				out.add(new Term.Line().add(Term.pad("reach", 6), Theme.MUTED)
						.add(lastReach < 0 ? "-" : String.format("%.2f", lastReach), Theme.TEXT));
			}
			return out;
		}
	}

	/** Held item durability, counts of useful items, ender pearl cooldown. */
	public static final class ItemsHud extends HudModule.Text {
		private final Setting.Bool held = add(new Setting.Bool("held", "held_durability", true));
		private final Setting.Bool counts = add(new Setting.Bool("counts", "item_counts", true));
		private final Setting.Bool cooldown = add(new Setting.Bool("cooldown", "pearl_cooldown", true));

		public ItemsHud() {
			super("items", "Items", "Held item durability, arrows, totems, pearls, gapples.", false, "Bottom right");
		}

		@Override
		protected List<Term.Line> lines(Minecraft mc) {
			if (mc.player == null) {
				return List.of();
			}
			Inventory inv = mc.player.getInventory();
			List<Term.Line> out = new ArrayList<>();
			ItemStack hand = mc.player.getMainHandItem();
			if (held.get() && hand.isDamageableItem() && hand.getMaxDamage() > 0) {
				int left = hand.getMaxDamage() - hand.getDamageValue();
				double frac = (double) left / hand.getMaxDamage();
				out.add(new Term.Line().add(Term.pad("held", 8), Theme.MUTED)
						.add(left + "/" + hand.getMaxDamage(), frac < 0.25 ? Theme.RED : frac < 0.5 ? Theme.AMBER : Theme.TEXT));
			}
			if (counts.get()) {
				count(out, inv, "arrows", Items.ARROW, Items.SPECTRAL_ARROW, Items.TIPPED_ARROW);
				count(out, inv, "totems", Items.TOTEM_OF_UNDYING);
				count(out, inv, "pearls", Items.ENDER_PEARL);
				count(out, inv, "gapples", Items.GOLDEN_APPLE, Items.ENCHANTED_GOLDEN_APPLE);
			}
			if (cooldown.get()) {
				//? if >=26 {
				float pct = mc.player.getCooldowns().getCooldownPercent(new ItemStack(Items.ENDER_PEARL), 0f);
				//?} else {
				/*float pct = mc.player.getCooldowns().getCooldownPercent(Items.ENDER_PEARL, 0f);
				*///?}
				if (pct > 0) {
					out.add(new Term.Line().add(Term.pad("pearl", 8), Theme.MUTED).append(Term.bar(1 - pct, 8, Theme.AMBER)));
				}
			}
			return out;
		}

		private static void count(List<Term.Line> out, Inventory inv, String label, net.minecraft.world.item.Item... items) {
			int n = 0;
			for (int i = 0; i < inv.getContainerSize(); i++) {
				ItemStack s = inv.getItem(i);
				for (var item : items) {
					if (s.is(item)) {
						n += s.getCount();
					}
				}
			}
			if (n > 0) {
				out.add(new Term.Line().add(Term.pad(label, 8), Theme.MUTED).add(String.valueOf(n), Theme.TEXT));
			}
		}
	}

	/** The resource packs you have on. */
	public static final class PackDisplay extends HudModule.Text {
		public PackDisplay() {
			super("pack_display", "Pack Display", "Lists the resource packs you have on.", false, "Bottom left");
		}

		@Override
		protected List<Term.Line> lines(Minecraft mc) {
			List<Term.Line> out = new ArrayList<>();
			for (Pack p : mc.getResourcePackRepository().getSelectedPacks()) {
				String id = p.getId();
				if (id.equals("vanilla") || id.startsWith("fabric") || id.equals("server")) {
					continue; // the built-in ones
				}
				out.add(new Term.Line().add("> ", Theme.accent()).add(p.getTitle().getString(), Theme.TEXT));
			}
			if (!out.isEmpty()) {
				out.addFirst(new Term.Line().add("# packs", Theme.MUTED));
			}
			return out;
		}
	}
}
