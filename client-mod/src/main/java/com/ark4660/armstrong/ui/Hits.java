package com.ark4660.armstrong.ui;

import java.util.ArrayList;
import java.util.List;

/** Click targets collected while drawing; the last added wins where they overlap. */
public final class Hits {
	private record Hit(int x, int y, int w, int h, Runnable action) {}

	private final List<Hit> hits = new ArrayList<>();

	public void clear() {
		hits.clear();
	}

	public void add(int x, int y, int w, int h, Runnable action) {
		hits.add(new Hit(x, y, w, h, action));
	}

	/** Runs the target under the point, if any. */
	public boolean click(double px, double py) {
		for (int i = hits.size() - 1; i >= 0; i--) {
			Hit h = hits.get(i);
			if (px >= h.x && px < h.x + h.w && py >= h.y && py < h.y + h.h) {
				h.action.run();
				return true;
			}
		}
		return false;
	}
}
