package com.ark4660.armstrong.waypoints;

import java.util.List;

import com.ark4660.armstrong.compat.Mc;
import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.HudOverlay;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.minecraft.client.option.KeyBinding;
import net.minecraft.client.MinecraftClient;
import com.ark4660.armstrong.compat.Gfx;
import net.minecraft.util.math.Vec3d;

/**
 * Named spots per world, shown as markers with their distance. One key adds
 * one where you stand, another opens the list. Markers are projected onto
 * the HUD from the camera, so they work the same on every version.
 */
public final class Waypoints extends Module implements HudOverlay {
	private static final int EDGE = 12; // markers off screen stay this far in

	private final KeyBinding addKey;
	private final KeyBinding listKey;
	private final WaypointStore store;
	private final Setting.Bool distance = add(new Setting.Bool("distance", "show_distance", true));
	private final Setting.Bool offscreen = add(new Setting.Bool("offscreen", "show_off_screen", true));
	private final Setting.Bool deathPoint = add(new Setting.Bool("death_point", "mark_death_point", true));

	private boolean wasDead;

	public Waypoints(KeyBinding addKey, KeyBinding listKey, WaypointStore store) {
		super("waypoints", "Waypoints", "Mark spots and see how far they are.", Category.VIEW, true);
		this.addKey = addKey;
		this.listKey = listKey;
		this.store = store;
	}

	@Override
	public KeyBinding key() {
		return addKey;
	}

	public WaypointStore store() {
		return store;
	}

	/** Adds a waypoint where the player stands; returns it, or null outside a world. */
	public Waypoint addHere(MinecraftClient mc, String name) {
		String key = WaypointStore.currentKey(mc);
		if (key == null || mc.player == null || mc.world == null) {
			return null;
		}
		List<Waypoint> list = store.list(key);
		Waypoint w = new Waypoint(name, (int) Math.floor(mc.player.x), (int) Math.floor(mc.player.getBoundingBox().minY),
				(int) Math.floor(mc.player.z), Mc.dimension(mc.world), list.size());
		list.add(w);
		store.save();
		return w;
	}

	@Override
	public void tick(MinecraftClient mc) {
		while (addKey.wasPressed()) {
			if (Mc.screen() == null && mc.player != null) {
				int n = store.list(WaypointStore.currentKey(mc)).size() + 1;
				Mc.open(new WaypointsScreen(this, addHere(mc, "waypoint " + n)));
			}
		}
		while (listKey.wasPressed()) {
			if (Mc.screen() == null && mc.player != null) {
				Mc.open(new WaypointsScreen(this, null));
			}
		}
		if (mc.player == null) {
			wasDead = false;
			return;
		}
		boolean dead = mc.player.getHealth() <= 0;
		if (dead && !wasDead && deathPoint.get()) {
			String key = WaypointStore.currentKey(mc);
			if (key != null) {
				store.list(key).removeIf(w -> w.name.equals("death"));
				Waypoint w = addHere(mc, "death");
				if (w != null) {
					w.color = 3; // red
					store.save();
				}
			}
		}
		wasDead = dead;
	}

	@Override
	public void drawOverlay(Gfx g, MinecraftClient mc) {
		if (mc.player == null || mc.world == null) {
			return;
		}
		String key = WaypointStore.currentKey(mc);
		if (key == null) {
			return;
		}
		String dim = Mc.dimension(mc.world);
		Mc.View v = Mc.view();
		double yaw = Math.toRadians(v.yaw()), pitch = Math.toRadians(v.pitch());
		// Camera axes: forward, right and up, in world space.
		double fx = -Math.sin(yaw) * Math.cos(pitch), fy = -Math.sin(pitch), fz = Math.cos(yaw) * Math.cos(pitch);
		double rx = -Math.cos(yaw), rz = -Math.sin(yaw);
		double ux = -rz * fy, uy = rz * fx - rx * fz, uz = rx * fy; // up = right × forward
		int w = g.guiWidth(), h = g.guiHeight();
		double focal = (h / 2.0) / Math.tan(Math.toRadians(Mc.fov) / 2);
		for (Waypoint p : store.list(key)) {
			if (!p.visible || !dim.equals(p.dimension)) {
				continue;
			}
			Vec3d d = new Vec3d(p.x + 0.5 - v.pos().x, p.y + 1.0 - v.pos().y, p.z + 0.5 - v.pos().z);
			double cx = d.x * rx + d.z * rz;
			double cy = d.x * ux + d.y * uy + d.z * uz;
			double cz = d.x * fx + d.y * fy + d.z * fz;
			double sx, sy;
			boolean inside;
			if (cz > 0.1) {
				sx = w / 2.0 + cx * focal / cz;
				sy = h / 2.0 - cy * focal / cz;
				inside = sx >= EDGE && sx <= w - EDGE && sy >= EDGE && sy <= h - EDGE;
			} else {
				// Behind the camera: towards its side, or the bottom if straight behind.
				boolean straight = Math.abs(cx) < 0.5 && Math.abs(cy) < 0.5;
				sx = w / 2.0 + (straight ? 0 : cx * 1e4);
				sy = h / 2.0 + (straight ? 1e4 : -cy * 1e4);
				inside = false;
			}
			if (!inside) {
				if (!offscreen.get()) {
					continue;
				}
				// Onto the screen edge, along the line from the centre.
				double dx = sx - w / 2.0, dy = sy - h / 2.0;
				double t = Math.min((w / 2.0 - EDGE) / Math.max(Math.abs(dx), 1e-6), (h / 2.0 - EDGE) / Math.max(Math.abs(dy), 1e-6));
				sx = w / 2.0 + dx * t;
				sy = h / 2.0 + dy * t;
			}
			marker(g, (int) Math.round(sx), (int) Math.round(sy), p, d.length(), inside);
		}
	}

	private void marker(Gfx g, int x, int y, Waypoint p, double dist, boolean inside) {
		int c = p.argb();
		int alpha = inside ? 0xFF000000 : 0xA0000000;
		int col = (c & 0x00FFFFFF) | alpha;
		// A small diamond.
		g.fill(x - 1, y - 3, x + 2, y + 4, col);
		g.fill(x - 3, y - 1, x + 4, y + 2, col);
		String label = p.name + (distance.get() ? " " + Math.round(dist) + "m" : "");
		int lw = Term.width(label);
		int lx = Math.clamp(x - lw / 2, 2, g.guiWidth() - lw - 2);
		int ly = y > g.guiHeight() - 16 ? y - 15 : y + 6;
		g.fill(lx - 2, ly - 1, lx + lw + 2, ly + 9, Theme.PANEL);
		Term.draw(g, label, lx, ly, inside ? Theme.TEXT : Theme.MUTED);
	}
}
