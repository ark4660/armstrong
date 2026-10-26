package com.ark4660.armstrong.module;

import com.google.gson.JsonElement;
import com.google.gson.JsonPrimitive;

/**
 * A module option the menu can show and the config can save. Kinds: on/off,
 * a number in a range, or one of a list of choices.
 */
public abstract sealed class Setting<T> permits Setting.Bool, Setting.Number, Setting.Choice, Setting.Text {
	public final String id;
	public final String name;
	protected final T defaultValue;
	protected T value;

	protected Setting(String id, String name, T defaultValue) {
		this.id = id;
		this.name = name;
		this.defaultValue = defaultValue;
		this.value = defaultValue;
	}

	public T get() {
		return value;
	}

	public void set(T value) {
		this.value = value;
	}

	public void reset() {
		value = defaultValue;
	}

	/** Text shown next to the name in the menu. */
	public abstract String display();

	abstract JsonElement toJson();

	/** Loads a saved value; anything unreadable or out of range keeps the current value. */
	abstract void fromJson(JsonElement json);

	public static final class Bool extends Setting<Boolean> {
		public Bool(String id, String name, boolean defaultValue) {
			super(id, name, defaultValue);
		}

		public void toggle() {
			value = !value;
		}

		@Override
		public String display() {
			return value ? "On" : "Off";
		}

		@Override
		JsonElement toJson() {
			return new JsonPrimitive(value);
		}

		@Override
		void fromJson(JsonElement json) {
			if (json.isJsonPrimitive() && json.getAsJsonPrimitive().isBoolean()) {
				value = json.getAsBoolean();
			}
		}
	}

	public static final class Number extends Setting<Double> {
		public final double min, max, step;
		private final String unit;

		public Number(String id, String name, double defaultValue, double min, double max, double step, String unit) {
			super(id, name, defaultValue);
			this.min = min;
			this.max = max;
			this.step = step;
			this.unit = unit;
		}

		@Override
		public void set(Double v) {
			double snapped = Math.round((v - min) / step) * step + min;
			value = Math.clamp(snapped, min, max);
		}

		/** Position in the range, 0 to 1, for sliders. */
		public double fraction() {
			return max == min ? 0 : (value - min) / (max - min);
		}

		public void setFraction(double f) {
			set(min + Math.clamp(f, 0, 1) * (max - min));
		}

		@Override
		public String display() {
			String n = step >= 1 ? String.valueOf(Math.round(value)) : String.format("%.1f", value);
			return n + unit;
		}

		@Override
		JsonElement toJson() {
			return new JsonPrimitive(value);
		}

		@Override
		void fromJson(JsonElement json) {
			if (json.isJsonPrimitive() && json.getAsJsonPrimitive().isNumber()) {
				set(json.getAsDouble());
			}
		}
	}

	/** Free text, like a chat message for a hotkey. */
	public static final class Text extends Setting<String> {
		public final int maxLength;

		public Text(String id, String name, String defaultValue, int maxLength) {
			super(id, name, defaultValue);
			this.maxLength = maxLength;
		}

		@Override
		public void set(String v) {
			value = v.length() > maxLength ? v.substring(0, maxLength) : v;
		}

		@Override
		public String display() {
			return value.isEmpty() ? "(empty)" : "\"" + value + "\"";
		}

		@Override
		JsonElement toJson() {
			return new JsonPrimitive(value);
		}

		@Override
		void fromJson(JsonElement json) {
			if (json.isJsonPrimitive()) {
				set(json.getAsString());
			}
		}
	}

	public static final class Choice extends Setting<String> {
		public final String[] options;

		public Choice(String id, String name, String defaultValue, String... options) {
			super(id, name, defaultValue);
			this.options = options;
		}

		public void next() {
			int i = indexOf(value);
			value = options[(i + 1) % options.length];
		}

		private int indexOf(String v) {
			for (int i = 0; i < options.length; i++) {
				if (options[i].equals(v)) {
					return i;
				}
			}
			return 0;
		}

		@Override
		public String display() {
			return value;
		}

		@Override
		JsonElement toJson() {
			return new JsonPrimitive(value);
		}

		@Override
		void fromJson(JsonElement json) {
			if (json.isJsonPrimitive()) {
				String v = json.getAsString();
				for (String o : options) {
					if (o.equals(v)) {
						value = v;
					}
				}
			}
		}
	}
}
