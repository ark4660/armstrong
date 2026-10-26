package com.ark4660.armstrong.modules;

import java.util.ArrayList;
import java.util.List;

import com.ark4660.armstrong.module.HudModule;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.option.KeyBinding;
import net.minecraft.client.resource.ResourcePackLoader;
import net.minecraft.entity.Entity;
import net.minecraft.entity.player.PlayerInventory;
import net.minecraft.item.Item;
import net.minecraft.item.ItemStack;
import net.minecraft.item.Items;
import net.minecraft.util.math.Box;
import net.minecraft.util.math.Vec3d;

/** Small text HUD panels. */
public final class InfoHuds {
	private InfoHuds() {}

	/** Press the key to start, again to stop, a third time to reset. */
	public static final class Stopwatch extends HudModule.Text {
		private final KeyBinding key;
		private long started, elapsed;
		private boolean running;

		public Stopwatch(KeyBinding key) {
			super("stopwatch", "Stopwatch", "Press the key: start, stop, reset.", false, "Top left");
			this.key = key;
		}

		@Override
		public KeyBinding key() {
			return key;
		}

		@Override
		public void tick(MinecraftClient mc) {
			while (key.wasPressed()) {
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
		protected List<Term.Line> lines(MinecraftClient mc) {
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
		public void onAttack(MinecraftClient mc, Entity target) {
			if (mc.player == null) {
				return;
			}
			hits = target.getEntityId() == lastTarget && System.currentTimeMillis() - lastHit < 3000 ? hits + 1 : 1;
			lastTarget = target.getEntityId();
			lastHit = System.currentTimeMillis();
			Vec3d eye = mc.player.getCameraPosVec(1f);
			Box box = target.getBoundingBox();
			double dx = Math.max(Math.max(box.minX - eye.x, 0), eye.x - box.maxX);
			double dy = Math.max(Math.max(box.minY - eye.y, 0), eye.y - box.maxY);
			double dz = Math.max(Math.max(box.minZ - eye.z, 0), eye.z - box.maxZ);
			lastReach = Math.sqrt(dx * dx + dy * dy + dz * dz);
		}

		@Override
		public void tick(MinecraftClient mc) {
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
		protected List<Term.Line> lines(MinecraftClient mc) {
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

	/** Held item durability and counts of useful items (no pearl cooldown or totems in 1.8.9). */
	public static final class ItemsHud extends HudModule.Text {
		private final Setting.Bool held = add(new Setting.Bool("held", "held_durability", true));
		private final Setting.Bool counts = add(new Setting.Bool("counts", "item_counts", true));

		public ItemsHud() {
			super("items", "Items", "Held item durability, arrows, pearls, gapples.", false, "Bottom right");
		}

		@Override
		protected List<Term.Line> lines(MinecraftClient mc) {
			if (mc.player == null) {
				return List.of();
			}
			PlayerInventory inv = mc.player.inventory;
			List<Term.Line> out = new ArrayList<>();
			ItemStack hand = inv.getMainHandStack();
			if (held.get() && hand != null && hand.isDamageable() && hand.getMaxDamage() > 0) {
				int left = hand.getMaxDamage() - hand.getDamage();
				double frac = (double) left / hand.getMaxDamage();
				out.add(new Term.Line().add(Term.pad("held", 8), Theme.MUTED)
						.add(left + "/" + hand.getMaxDamage(), frac < 0.25 ? Theme.RED : frac < 0.5 ? Theme.AMBER : Theme.TEXT));
			}
			if (counts.get()) {
				count(out, inv, "arrows", Items.ARROW);
				count(out, inv, "pearls", Items.ENDER_PEARL);
				count(out, inv, "gapples", Items.GOLDEN_APPLE); // enchanted ones are the same item
			}
			return out;
		}

		private static void count(List<Term.Line> out, PlayerInventory inv, String label, Item item) {
			int n = 0;
			for (int i = 0; i < inv.getInvSize(); i++) {
				ItemStack s = inv.getInvStack(i);
				if (s != null && s.getItem() == item) {
					n += s.count;
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
		protected List<Term.Line> lines(MinecraftClient mc) {
			List<Term.Line> out = new ArrayList<>();
			for (ResourcePackLoader.Entry p : mc.getResourcePackLoader().getSelectedResourcePacks()) {
				String name = p.getName().replaceFirst("\\.zip$", "");
				out.add(new Term.Line().add("> ", Theme.accent()).add(name, Theme.TEXT));
			}
			if (!out.isEmpty()) {
				out.add(0, new Term.Line().add("# packs", Theme.MUTED));
			}
			return out;
		}
	}
}
