package com.ark4660.armstrong.module;

public enum Category {
	HUD("hud"),
	MOVEMENT("movement"),
	VIEW("view"),
	CLIENT("client");

	public final String title;

	Category(String title) {
		this.title = title;
	}
}
