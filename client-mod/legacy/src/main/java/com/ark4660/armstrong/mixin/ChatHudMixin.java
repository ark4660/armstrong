package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.Shadow;
import org.spongepowered.asm.mixin.Unique;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Constant;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.ModifyConstant;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ChatModules;

import net.minecraft.client.gui.hud.ChatHud;
import net.minecraft.text.Text;

/**
 * The Chat module: timestamps, [xN] for repeats, and a longer history. Each
 * message gets its own chat line id, so a repeat can replace the previous
 * line through the game's own "replace the line with this id".
 */
@Mixin(ChatHud.class)
public abstract class ChatHudMixin {
	@Unique
	private static final int FIRST_ID = 0x41520000; // clear of the few ids vanilla uses

	@Unique
	private int armstrong$lineId = FIRST_ID;

	@Shadow
	public abstract void addMessage(Text message, int messageId);

	@Inject(method = "addMessage(Lnet/minecraft/text/Text;)V", at = @At("HEAD"), cancellable = true)
	private void armstrong$decorate(Text message, CallbackInfo ci) {
		ChatModules.Chat chat = ArmstrongClient.active(ChatModules.Chat.class);
		if (chat == null) {
			return;
		}
		if (!chat.isRepeat(message)) {
			armstrong$lineId = armstrong$lineId == Integer.MAX_VALUE ? FIRST_ID : armstrong$lineId + 1;
		}
		addMessage(chat.decorate(message), armstrong$lineId);
		ci.cancel();
	}

	@ModifyConstant(method = "addMessage(Lnet/minecraft/text/Text;IIZ)V", constant = @Constant(intValue = 100))
	private int armstrong$history(int vanilla) {
		ChatModules.Chat chat = ArmstrongClient.active(ChatModules.Chat.class);
		return chat == null ? vanilla : chat.history();
	}
}
