package com.ark4660.armstrong.module;

import net.minecraft.client.Minecraft;
import net.minecraft.client.gui.GuiGraphicsExtractor;

/** A module that draws across the HUD rather than in a panel (waypoint markers). */
public interface HudOverlay {
	void drawOverlay(GuiGraphicsExtractor g, Minecraft mc);
}
