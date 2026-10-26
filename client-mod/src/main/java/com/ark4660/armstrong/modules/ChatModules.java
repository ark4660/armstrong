package com.ark4660.armstrong.modules;

import java.time.LocalTime;
import java.time.format.DateTimeFormatter;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.ChatFormatting;
import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;
import net.minecraft.network.chat.Component;

public final class ChatModules {
	private ChatModules() {}

	/** Timestamps, stacked repeats ([x3]) and a longer history. */
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
		public boolean isRepeat(Component message) {
			return stack.get() && message.getString().equals(last);
		}

		/** The message as shown: with a timestamp and a repeat count. */
		public Component decorate(Component message) {
			String plain = message.getString();
			repeats = plain.equals(last) && stack.get() ? repeats + 1 : 1;
			last = plain;
			Component out = message;
			if (repeats > 1) {
				out = Component.empty().append(message).append(Component.literal(" [x" + repeats + "]").withStyle(ChatFormatting.GRAY));
			}
			if (timestamps.get()) {
				String t = LocalTime.now().format(clock.get().equals("12h") ? H12 : H24);
				out = Component.literal("[" + t + "] ").withStyle(ChatFormatting.DARK_GRAY).append(out);
			}
			return out;
		}
	}

	/** Keys that send a chat message or command, like "gg" or "/spawn". */
	public static final class AutoText extends Module {
		private final KeyMapping[] keys;
		private final Setting.Text[] messages;

		public AutoText(KeyMapping... keys) {
			super("auto_text", "Auto Text", "Keys that send a message or /command (set the keys in controls).", Category.CHAT, false);
			this.keys = keys;
			messages = new Setting.Text[keys.length];
			String[] defaults = {"gg", "", ""};
			for (int i = 0; i < keys.length; i++) {
				messages[i] = add(new Setting.Text("text_" + (i + 1), "text_" + (i + 1), i < defaults.length ? defaults[i] : "", 100));
			}
		}

		@Override
		public KeyMapping key() {
			return keys[0];
		}

		@Override
		public void tick(Minecraft mc) {
			for (int i = 0; i < keys.length; i++) {
				while (keys[i].consumeClick()) {
					String text = messages[i].get().strip();
					if (text.isEmpty() || mc.player == null || mc.getConnection() == null || mc.gui.screen() != null) {
						continue;
					}
					if (text.startsWith("/")) {
						mc.getConnection().sendCommand(text.substring(1));
					} else {
						mc.getConnection().sendChat(text);
					}
				}
			}
		}
	}
}
