package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.gen.Invoker;

import net.minecraft.client.gui.screen.TitleScreen;

/** The title screen's panorama (blurred, rotating), for the home screens' background. */
@Mixin(TitleScreen.class)
public interface TitleScreenAccessor {
	@Invoker("renderBackground")
	void armstrong$renderPanorama(int mouseX, int mouseY, float tickDelta);
}
