package com.ark4660.armstrong.replay;

import com.ark4660.armstrong.module.Category;
import com.ark4660.armstrong.module.Indicator;
import com.ark4660.armstrong.module.Module;
import com.ark4660.armstrong.module.Setting;

import net.minecraft.client.KeyMapping;
import net.minecraft.client.Minecraft;

/**
 * Replays, powered by Flashback: a key to start and stop recording, optional
 * recording from the moment you join, and a [rec] tag while it runs. Watch
 * replays from the home screen ("replays").
 */
public final class ReplayModule extends Module implements Indicator {
	private final KeyMapping key;
	private final Setting.Bool autoRecord = add(new Setting.Bool("auto_record", "record_on_join", false));
	private final Setting.Bool showTag = add(new Setting.Bool("show_tag", "show_rec_tag", true));
	private long since;
	private boolean wasRecording;

	public ReplayModule(KeyMapping key) {
		super("replay", "Replay", "Record and re-watch your games (needs the Flashback add-on).", Category.REPLAY, true);
		this.key = key;
	}

	@Override
	public KeyMapping key() {
		return key;
	}

	/** Called when you join a world or server. */
	public void onJoin() {
		if (isActive() && autoRecord.get() && Flashback.installed() && !Flashback.recording() && !Flashback.inReplay()) {
			Flashback.start();
		}
	}

	@Override
	public void tick(Minecraft mc) {
		while (key.consumeClick()) {
			if (!Flashback.installed()) {
				if (mc.player != null) {
					mc.player.sendSystemMessage(net.minecraft.network.chat.Component.literal(
							"Replays need the Flashback add-on: launcher → Mods → Client add-ons."));
				}
			} else if (Flashback.recording()) {
				Flashback.finish();
			} else if (mc.player != null && !Flashback.inReplay()) {
				Flashback.start();
			}
		}
		boolean rec = Flashback.recording();
		if (rec && !wasRecording) {
			since = System.currentTimeMillis();
		}
		wasRecording = rec;
	}

	@Override
	public String indicatorLabel() {
		return wasRecording && showTag.get() ? "rec" : null;
	}

	@Override
	public String indicatorValue() {
		long s = (System.currentTimeMillis() - since) / 1000;
		return String.format("%02d:%02d", s / 60, s % 60);
	}
}
