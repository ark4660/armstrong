package com.ark4660.armstrong.compat;

import org.lwjgl.input.Keyboard;
import org.lwjgl.input.Mouse;

import net.minecraft.client.gui.screen.Screen;
import net.minecraft.util.SharedConstants;

/**
 * The 1.8.9 version of the modern builds' CompatScreen: the same hooks
 * (onKey, onChar, onClick, onDrag, onRelease, mouseScrolled, draw), so
 * screens port by renaming types. 1.8.9 delivers a key and its character
 * together; while typing is on, printable characters go to onChar first.
 */
public abstract class CompatScreen extends Screen {
	private boolean typing;
	private int lastX, lastY;

	/** A key press (LWJGL 2 code); modifiers is 1 with Shift held. Return false if unused. */
	protected boolean onKey(int key, int modifiers) {
		return false;
	}

	/** A typed character; return false if unused. */
	protected boolean onChar(String c, boolean allowedInChat) {
		return false;
	}

	protected boolean onClick(double x, double y, int button) {
		return false;
	}

	protected boolean onDrag(double x, double y, int button, double dx, double dy) {
		return false;
	}

	protected boolean onRelease(double x, double y, int button) {
		return false;
	}

	/** The mouse wheel at GUI coordinates (vertical > 0 is up). */
	public boolean mouseScrolled(double x, double y, double horizontal, double vertical) {
		return false;
	}

	/** Draws the background; by default the game's own. */
	protected void drawBackground(Gfx g, int mouseX, int mouseY, float delta) {
		vanillaBackground(g, mouseX, mouseY, delta);
	}

	/** Draws the screen, after the game's widgets. */
	protected void draw(Gfx g, int mouseX, int mouseY, float delta) {}

	/** Printable keys count as text while this is on (search boxes, renaming). */
	protected void startTyping() {
		typing = true;
		Keyboard.enableRepeatEvents(true);
	}

	protected void stopTyping() {
		typing = false;
		Keyboard.enableRepeatEvents(false);
	}

	protected void vanillaBackground(Gfx g, int mouseX, int mouseY, float delta) {
		renderBackground();
	}

	public boolean isPauseScreen() {
		return true;
	}

	public boolean shouldCloseOnEsc() {
		return true;
	}

	/** Closes the screen (Escape when shouldCloseOnEsc). */
	public void onClose() {
		Mc.open(null);
	}

	@Override
	public boolean shouldPauseGame() {
		return isPauseScreen();
	}

	@Override
	protected void keyPressed(char c, int code) {
		boolean printable = c >= ' ' && c != 127;
		if (typing && printable && onChar(String.valueOf(c), SharedConstants.isValidChar(c))) {
			return;
		}
		if (onKey(code, hasShiftDown() ? 1 : 0)) {
			return;
		}
		if (printable && onChar(String.valueOf(c), SharedConstants.isValidChar(c))) {
			return;
		}
		if (code == Keyboard.KEY_ESCAPE && shouldCloseOnEsc()) {
			onClose();
		}
	}

	@Override
	protected void mouseClicked(int x, int y, int button) {
		lastX = x;
		lastY = y;
		if (!onClick(x, y, button)) {
			super.mouseClicked(x, y, button);
		}
	}

	@Override
	protected void mouseDragged(int x, int y, int button, long time) {
		onDrag(x, y, button, x - lastX, y - lastY);
		lastX = x;
		lastY = y;
	}

	@Override
	protected void mouseReleased(int x, int y, int button) {
		if (!onRelease(x, y, button)) {
			super.mouseReleased(x, y, button);
		}
	}

	@Override
	public void handleMouse() {
		super.handleMouse();
		int wheel = Mouse.getEventDWheel();
		if (wheel != 0) {
			double x = Mouse.getEventX() * (double) width / client.width;
			double y = height - Mouse.getEventY() * (double) height / client.height - 1;
			mouseScrolled(x, y, 0, Math.signum(wheel));
		}
	}

	@Override
	public void removed() {
		if (typing) {
			stopTyping();
		}
	}

	@Override
	public void render(int mouseX, int mouseY, float delta) {
		Gfx g = new Gfx();
		drawBackground(g, mouseX, mouseY, delta);
		super.render(mouseX, mouseY, delta);
		draw(g, mouseX, mouseY, delta);
	}
}
