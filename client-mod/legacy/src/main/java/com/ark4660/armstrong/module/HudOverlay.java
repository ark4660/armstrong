package com.ark4660.armstrong.module;

import net.minecraft.client.MinecraftClient;
import com.ark4660.armstrong.compat.Gfx;

/** A module that draws across the HUD rather than in a panel (waypoint markers). */
public interface HudOverlay {
	void drawOverlay(Gfx g, MinecraftClient mc);
}
