package com.ark4660.armstrong.waypoints;

/** A named spot in one dimension of one world or server. */
public final class Waypoint {
	/** Marker colours (ARGB), picked by index. */
	public static final int[] COLORS = {0xFF5EF38C, 0xFFFFB454, 0xFF58C4FF, 0xFFFF6B6B, 0xFFC792EA, 0xFFFFFFFF};

	public String name;
	public int x, y, z;
	public String dimension;
	public int color;
	public boolean visible = true;

	public Waypoint(String name, int x, int y, int z, String dimension, int color) {
		this.name = name;
		this.x = x;
		this.y = y;
		this.z = z;
		this.dimension = dimension;
		this.color = color;
	}

	public int argb() {
		return COLORS[Math.floorMod(color, COLORS.length)];
	}
}
