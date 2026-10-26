package com.ark4660.armstrong.modules;

import com.ark4660.armstrong.compat.Mc;

import java.time.LocalTime;
import java.time.format.DateTimeFormatter;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.client.MinecraftClient;
import net.minecraft.client.option.KeyBinding;
import net.minecraft.text.LiteralText;
import net.minecraft.text.Style;
import net.minecraft.text.Text;
import net.minecraft.util.Formatting;

public final class ChatModules {
	private ChatModules() {}

	/** Timestamps, stacked repeats ([x3]) and a longer history (see ChatHudMixin). */
	public static final class Chat extends Module {
		private static final DateTimeFormatter H24 = DateTimeFormatter.ofPattern("HH:mm");
		private static final DateTimeFormatter H12 = DateTimeFormatter.ofPattern("h:mm a");

		private final Setting.Bool timestamps = add(new Setting.Bool("timestamps", "timestamps", true));
		private final Setting.Choice clock = add(new Setting.Choice("clock", "clock", "24h", "24h", "12h"));
		private final Setting.Bool stack = add(new Setting.Bool("stack", "stack_repeats", true));
		private final Setting.Number history = add(new Setting.Number("history", "history_lines", 500, 100, 2000, 100, ""));

		private String last;
		private int repeats;

		public Chat() {
			super("chat", "Chat", "Timestamps, stacked repeated messages, longer history.", Category.CHAT, true);
		}

		public int history() {
			return (int) Math.round(history.get());
		}

		/** Whether this message repeats the previous one (so the previous line gets replaced). */
		public boolean isRepeat(Text message) {
			return stack.get() && message.asUnformattedString().equals(last);
		}

		/** The message as shown: with a timestamp and a repeat count. */
		public Text decorate(Text message) {
			String plain = message.asUnformattedString();
			repeats = plain.equals(last) && stack.get() ? repeats + 1 : 1;
			last = plain;
			Text out = message;
			if (repeats > 1) {
				out = new LiteralText("").append(message).append(gray(" [x" + repeats + "]", Formatting.GRAY));
			}
			if (timestamps.get()) {
				String t = LocalTime.now().format(clock.get().equals("12h") ? H12 : H24);
				out = gray("[" + t + "] ", Formatting.DARK_GRAY).append(out);
			}
			return out;
		}

		private static Text gray(String s, Formatting f) {
			return new LiteralText(s).setStyle(new Style().setFormatting(f));
		}
	}

	/** Keys that send a chat message or command, like "gg" or "/spawn". */
	public static final class AutoText extends Module {
		private final KeyBinding[] keys;
		private final Setting.Text[] messages;

		public AutoText(KeyBinding... keys) {
			super("auto_text", "Auto Text", "Keys that send a message or /command (set the keys in controls).", Category.CHAT, false);
			this.keys = keys;
			messages = new Setting.Text[keys.length];
			String[] defaults = {"gg", "", ""};
			for (int i = 0; i < keys.length; i++) {
				messages[i] = add(new Setting.Text("text_" + (i + 1), "text_" + (i + 1), i < defaults.length ? defaults[i] : "", 100));
			}
		}

		@Override
		public KeyBinding key() {
			return keys[0];
		}

		@Override
		public void tick(MinecraftClient mc) {
			for (int i = 0; i < keys.length; i++) {
				while (keys[i].wasPressed()) {
					String text = messages[i].get().strip();
					if (text.isEmpty() || mc.player == null || Mc.screen() != null) {
						continue;
					}
					mc.player.sendChatMessage(text); // commands are chat messages before 1.19
				}
			}
		}
	}
}
