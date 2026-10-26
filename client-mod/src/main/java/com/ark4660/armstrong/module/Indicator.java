package com.ark4660.armstrong.module;

/** A module that shows a small status tag above the hotbar, like [sprint: toggled]. */
public interface Indicator {
	/** The tag's text, or null to show nothing right now. */
	String indicatorLabel();

	String indicatorValue();
}
