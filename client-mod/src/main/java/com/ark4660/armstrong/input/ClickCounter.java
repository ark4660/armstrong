package com.ark4660.armstrong.input;

import java.util.ArrayDeque;

/** Counts mouse clicks per second (CPS) for the left and right buttons. */
public final class ClickCounter {
	private static final ArrayDeque<Long> left = new ArrayDeque<>();
	private static final ArrayDeque<Long> right = new ArrayDeque<>();

	private ClickCounter() {}

	public static void click(boolean isLeft) {
		ArrayDeque<Long> q = isLeft ? left : right;
		synchronized (q) {
			q.addLast(System.nanoTime());
		}
	}

	public static int left() {
		return count(left);
	}

	public static int right() {
		return count(right);
	}

	private static int count(ArrayDeque<Long> q) {
		long cutoff = System.nanoTime() - 1_000_000_000L;
		synchronized (q) {
			while (!q.isEmpty() && q.peekFirst() < cutoff) {
				q.pollFirst();
			}
			return q.size();
		}
	}
}
