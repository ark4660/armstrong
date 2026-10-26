package com.ark4660.armstrong.module;

import java.util.List;

import com.ark4660.armstrong.ui.Term;
import com.ark4660.armstrong.ui.Theme;

import net.minecraft.client.Minecraft;
import net.minecraft.client.gui.GuiGraphicsExtractor;

/**
 * A module that draws a terminal-style panel on the HUD. It's placed by an
 * anchor and offset; the drag-and-drop HUD editor will set the same settings.
 */
public abstract class HudModule extends Module {
	protected static final int PADDING = 4;

	protected final Setting.Choice anchor;
	protected final Setting.Number offsetX;
	protected final Setting.Number offsetY;
	protected final Setting.Bool background;

	protected HudModule(String id, String name, String description, boolean enabledByDefault, String defaultAnchor) {
		this(id, name, description, enabledByDefault, defaultAnchor, 6, 6);
	}

	/** With a default offset, so panels that share a corner don't overlap. */
	protected HudModule(String id, String name, String description, boolean enabledByDefault, String defaultAnchor, int x, int y) {
		super(id, name, description, Category.HUD, enabledByDefault);
		anchor = add(new Setting.Choice("anchor", "position", defaultAnchor,
				"Top left", "Top right", "Bottom left", "Bottom right"));
		offsetX = add(new Setting.Number("offset_x", "offset_x", x, 0, 300, 1, "px"));
		offsetY = add(new Setting.Number("offset_y", "offset_y", y, 0, 300, 1, "px"));
		background = add(new Setting.Bool("background", "background", true));
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
	 * much room it took, so the next panel in that corner goes after it.
	 */
	public final int render(GuiGraphicsExtractor g, Minecraft mc, int stacked) {
		int cw = width(mc);
		if (cw <= 0) {
			return 0;
		}
		int pad = ownBoxes() ? 0 : PADDING;
		int w = cw + 2 * pad;
		int h = height(mc) + 2 * pad;
		int ox = (int) Math.round(offsetX.get());
		int oy = (int) Math.round(offsetY.get()) + stacked;
		String a = anchor.get();
		int x = a.endsWith("right") ? g.guiWidth() - w - ox : ox;
		int y = a.startsWith("Bottom") ? g.guiHeight() - h - oy : oy;
		if (background.get() && !ownBoxes()) {
			Term.panel(g, x, y, w, h, Theme.PANEL, Theme.BORDER);
		}
		draw(g, mc, x + pad, y + pad);
		return h + (int) Math.round(offsetY.get()) - 2;
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

		private List<Term.Line> cached;

		@Override
		protected int width(Minecraft mc) {
			cached = lines(mc);
			int w = 0;
			for (Term.Line l : cached) {
				w = Math.max(w, l.width());
			}
			return w;
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
