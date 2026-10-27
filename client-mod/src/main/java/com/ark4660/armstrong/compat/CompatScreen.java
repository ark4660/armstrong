package com.ark4660.armstrong.compat;

import net.minecraft.client.gui.GuiGraphicsExtractor;
import net.minecraft.client.gui.screens.Screen;
import net.minecraft.network.chat.Component;
//? if >=1.21.11 {
import net.minecraft.client.input.CharacterEvent;
import net.minecraft.client.input.KeyEvent;
import net.minecraft.client.input.MouseButtonEvent;
//?}

/**
 * A screen with version-neutral input and drawing hooks. Minecraft 1.21.11
 * and later pass input as event objects (before: key codes and chars), and
 * 26.x draws through extract* methods (before: render*). Armstrong's screens
 * override the hooks below instead of any of them.
 */
public abstract class CompatScreen extends Screen {
	protected CompatScreen(Component title) {
		super(title);
	}

	/** A key press; return false to let the game handle it (Escape closes, etc). */
	protected boolean onKey(int key, int modifiers) {
		return false;
	}

	/** A typed character; return false to let the game handle it. */
	protected boolean onChar(String c, boolean allowedInChat) {
		return false;
	}

	/** A mouse click at GUI coordinates; return false to let the game handle it. */
	protected boolean onClick(double x, double y, int button) {
		return false;
	}

	/** The mouse moved with a button held; return false to let the game handle it. */
	protected boolean onDrag(double x, double y, int button, double dx, double dy) {
		return false;
	}

	/** A mouse button let go; return false to let the game handle it. */
	protected boolean onRelease(double x, double y, int button) {
		return false;
	}

	/** Draws the background; by default the game's own. */
	protected void drawBackground(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		vanillaBackground(g, mouseX, mouseY, delta);
	}

	/** Draws the screen, after the game's widgets. */
	protected void draw(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {}

	/** Typed text arrives only while this is on in 26.x (always before). */
	protected void startTyping() {
		//? if >=26
		minecraft.textInputManager().startTextInput(this);
	}

	protected void stopTyping() {
		//? if >=26
		minecraft.textInputManager().stopTextInput(this);
	}

	//? if >=26 {
	protected void vanillaBackground(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		super.extractBackground(g, mouseX, mouseY, delta);
	}

	protected void panorama(GuiGraphicsExtractor g, float delta) {
		extractPanorama(g, delta);
	}

	@Override
	public void extractBackground(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		drawBackground(g, mouseX, mouseY, delta);
	}

	@Override
	public void extractRenderState(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		super.extractRenderState(g, mouseX, mouseY, delta);
		draw(g, mouseX, mouseY, delta);
	}
	//?} else {
	/*protected void vanillaBackground(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		super.renderBackground(g, mouseX, mouseY, delta);
	}

	protected void panorama(GuiGraphicsExtractor g, float delta) {
		renderPanorama(g, delta);
	}

	@Override
	public void renderBackground(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		drawBackground(g, mouseX, mouseY, delta);
	}

	@Override
	public void render(GuiGraphicsExtractor g, int mouseX, int mouseY, float delta) {
		super.render(g, mouseX, mouseY, delta);
		draw(g, mouseX, mouseY, delta);
	}
	*///?}

	//? if >=1.21.11 {
	@Override
	public boolean keyPressed(KeyEvent event) {
		return onKey(event.key(), event.modifiers()) || super.keyPressed(event);
	}

	@Override
	public boolean charTyped(CharacterEvent event) {
		return onChar(event.codepointAsString(), event.isAllowedChatCharacter()) || super.charTyped(event);
	}

	@Override
	public boolean mouseClicked(MouseButtonEvent event, boolean doubleClick) {
		return onClick(event.x(), event.y(), event.button()) || super.mouseClicked(event, doubleClick);
	}

	@Override
	public boolean mouseDragged(MouseButtonEvent event, double dx, double dy) {
		return onDrag(event.x(), event.y(), event.button(), dx, dy) || super.mouseDragged(event, dx, dy);
	}

	@Override
	public boolean mouseReleased(MouseButtonEvent event) {
		return onRelease(event.x(), event.y(), event.button()) || super.mouseReleased(event);
	}
	//?} else {
	/*@Override
	public boolean keyPressed(int key, int scanCode, int modifiers) {
		return onKey(key, modifiers) || super.keyPressed(key, scanCode, modifiers);
	}

	@Override
	public boolean charTyped(char c, int modifiers) {
		return onChar(String.valueOf(c), net.minecraft.util.StringUtil.isAllowedChatCharacter(c)) || super.charTyped(c, modifiers);
	}

	@Override
	public boolean mouseClicked(double x, double y, int button) {
		return onClick(x, y, button) || super.mouseClicked(x, y, button);
	}

	@Override
	public boolean mouseDragged(double x, double y, int button, double dx, double dy) {
		return onDrag(x, y, button, dx, dy) || super.mouseDragged(x, y, button, dx, dy);
	}

	@Override
	public boolean mouseReleased(double x, double y, int button) {
		return onRelease(x, y, button) || super.mouseReleased(x, y, button);
	}
	*///?}
}
