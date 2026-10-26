package com.ark4660.armstrong.compat;

import net.minecraft.client.gui.GuiGraphicsExtractor;

/** 2D transforms for GUI drawing: a 2D matrix stack in 26.x, a 3D pose stack before. */
public final class Pose {
	private Pose() {}

	public static void push(GuiGraphicsExtractor g) {
		//? if >=26 {
		g.pose().pushMatrix();
		//?} else {
		/*g.pose().pushPose();
		*///?}
	}

	public static void pop(GuiGraphicsExtractor g) {
		//? if >=26 {
		g.pose().popMatrix();
		//?} else {
		/*g.pose().popPose();
		*///?}
	}

	public static void translate(GuiGraphicsExtractor g, float x, float y) {
		//? if >=26 {
		g.pose().translate(x, y);
		//?} else {
		/*g.pose().translate(x, y, 0f);
		*///?}
	}

	public static void scale(GuiGraphicsExtractor g, float s) {
		//? if >=26 {
		g.pose().scale(s, s);
		//?} else {
		/*g.pose().scale(s, s, 1f);
		*///?}
	}
}
