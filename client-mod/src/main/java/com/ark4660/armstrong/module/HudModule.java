package com.ark4660.armstrong.module;

import java.util.List;

import com.ark4660.armstrong.compat.Pose;
import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.minecraft.client.Minecraft;
import net.minecraft.client.gui.GuiGraphicsExtractor;

/**
 * A module that draws a terminal-style panel on the HUD. It's placed by an
 * anchor (the nearest corner) and an offset from it. Panels left where they
 * are stack in their corner; one dragged in the HUD editor gets a custom
 * position and stays exactly where it was dropped.
 */
public abstract class HudModule extends Module {
	protected static final int PADDING = 4;
	private static final int MAX_OFFSET = 2000;

	protected final Setting.Choice anchor;
	protected final Setting.Number offsetX;
	protected final Setting.Number offsetY;
	protected final Setting.Bool background;
	protected final Setting.Number scale;
	protected final Setting.Bool custom;

	/** Client ticks so far: text panels rebuild their lines once per tick. */
	static long ticks;

	/** Where the panel was last drawn, in GUI pixels (scaled); 0 width if hidden. For the HUD editor. */
	public int lastX, lastY, lastW, lastH;

	protected HudModule(String id, String name, String description, boolean enabledByDefault, String defaultAnchor) {
		this(id, name, description, enabledByDefault, defaultAnchor, 6, 6);
	}

	/** With a default offset, so panels that share a corner don't overlap. */
	protected HudModule(String id, String name, String description, boolean enabledByDefault, String defaultAnchor, int x, int y) {
		super(id, name, description, Category.HUD, enabledByDefault);
		anchor = add(new Setting.Choice("anchor", "position", defaultAnchor,
				"Top left", "Top right", "Bottom left", "Bottom right"));
		offsetX = add(new Setting.Number("offset_x", "offset_x", x, 0, MAX_OFFSET, 1, "px"));
		offsetY = add(new Setting.Number("offset_y", "offset_y", y, 0, MAX_OFFSET, 1, "px"));
		background = add(new Setting.Bool("background", "background", true));
		scale = add(new Setting.Number("scale", "scale", 100, 50, 200, 5, "%"));
		custom = add(new Setting.Bool("custom_position", "custom_position", false));
	}

	/** Size of the content in GUI pixels, without padding; 0 width hides the panel. */
	protected abstract int width(Minecraft mc);

	protected abstract int height(Minecraft mc);

	/** Draws the content with its top-left corner at x, y. */
	protected abstract void draw(GuiGraphicsExtractor g, Minecraft mc, int x, int y);

	/** Whether the content draws its own boxes (keystrokes) instead of one panel. */
	protected boolean ownBoxes() {
		return false;
	}

	/** The corner this panel stacks in. */
	public String anchor() {
		return anchor.get();
	}

	/**
	 * Draws the panel, pushed away from its corner by `stacked` pixels (the
	 * height of the panels drawn before it in the same corner). Returns how
	 * much room it took, so the next panel in that corner goes after it. A
	 * panel with a custom position ignores stacking and takes no room. In
	 * the editor, an empty panel shows its name so it can still be moved.
	 */
	public final int render(GuiGraphicsExtractor g, Minecraft mc, int stacked, boolean editing) {
		int cw = width(mc);
		Term.Line placeholder = null;
		if (cw <= 0) {
			if (!editing) {
				lastW = 0;
				return 0;
			}
			placeholder = new Term.Line().add("# " + id, Theme.MUTED);
			cw = placeholder.width();
		}
		boolean own = ownBoxes() && placeholder == null;
		int pad = own ? 0 : PADDING;
		int iw = cw + 2 * pad;
		int ih = (placeholder != null ? Term.LINE - 1 : height(mc)) + 2 * pad;
		float s = scale();
		int w = Math.round(iw * s);
		int h = Math.round(ih * s);
		boolean free = custom.get();
		int ox = (int) Math.round(offsetX.get());
		int oy = (int) Math.round(offsetY.get()) + (free ? 0 : stacked);
		String a = anchor.get();
		int x = a.endsWith("right") ? g.guiWidth() - w - ox : ox;
		int y = a.startsWith("Bottom") ? g.guiHeight() - h - oy : oy;
		lastX = x;
		lastY = y;
		lastW = w;
		lastH = h;

		Pose.push(g);
		Pose.translate(g, x, y);
		Pose.scale(g, s);
		if ((background.get() || placeholder != null) && !own) {
			Term.panel(g, 0, 0, iw, ih, Theme.PANEL, Theme.BORDER);
		}
		if (placeholder != null) {
			placeholder.draw(g, pad, pad);
		} else {
			draw(g, mc, pad, pad);
		}
		Pose.pop(g);
		return free ? 0 : h + (int) Math.round(offsetY.get()) - 2;
	}

	/**
	 * Puts the panel's top-left corner at x, y (from the HUD editor). The
	 * nearest corner becomes its anchor, so it keeps its place when the
	 * window is resized.
	 */
	public void place(int x, int y, int guiWidth, int guiHeight) {
		boolean right = x + lastW / 2 > guiWidth / 2;
		boolean bottom = y + lastH / 2 > guiHeight / 2;
		anchor.set((bottom ? "Bottom" : "Top") + " " + (right ? "right" : "left"));
		offsetX.set((double) Math.max(0, right ? guiWidth - x - lastW : x));
		offsetY.set((double) Math.max(0, bottom ? guiHeight - y - lastH : y));
		custom.set(true);
		lastX = x;
		lastY = y;
	}

	/** Back to the default corner, offset and size, stacking with the others. */
	public void resetPlacement() {
		anchor.reset();
		offsetX.reset();
		offsetY.reset();
		scale.reset();
		custom.reset();
	}

	/** The panel's size factor (1 = 100%). */
	public float scale() {
		return (float) (scale.get() / 100.0);
	}

	/** Grows or shrinks the panel by steps of 10%. */
	public void rescale(int steps) {
		scale.set(scale.get() + steps * 10);
	}

	/** A HUD module made of text lines, the common case. */
	public abstract static class Text extends HudModule {
		protected Text(String id, String name, String description, boolean enabledByDefault, String defaultAnchor) {
			super(id, name, description, enabledByDefault, defaultAnchor);
		}

		protected Text(String id, String name, String description, boolean enabledByDefault, String defaultAnchor, int x, int y) {
			super(id, name, description, enabledByDefault, defaultAnchor, x, y);
		}

		/** The lines to show; empty hides the panel. */
		protected abstract List<Term.Line> lines(Minecraft mc);

		/**
		 * Whether the lines change faster than the game ticks (a stopwatch's
		 * hundredths). Other panels are rebuilt once per tick, not every frame:
		 * fps, coordinates and ping don't change in between.
		 */
		protected boolean live() {
			return false;
		}

		private List<Term.Line> cached;
		private int cachedWidth;
		private long cachedTick = -1;

		@Override
		protected int width(Minecraft mc) {
			if (cached == null || cachedTick != ticks || live()) {
				cached = lines(mc);
				cachedTick = ticks;
				cachedWidth = 0;
				for (Term.Line l : cached) {
					cachedWidth = Math.max(cachedWidth, l.width());
				}
			}
			return cachedWidth;
		}

		@Override
		protected int height(Minecraft mc) {
			return cached == null || cached.isEmpty() ? 0 : cached.size() * Term.LINE - 1;
		}

		@Override
		protected void draw(GuiGraphicsExtractor g, Minecraft mc, int x, int y) {
			for (Term.Line l : cached) {
				l.draw(g, x, y);
				y += Term.LINE;
			}
		}
	}
}
