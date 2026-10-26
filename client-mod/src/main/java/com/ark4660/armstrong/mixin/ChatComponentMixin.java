package com.ark4660.armstrong.mixin;

import java.util.List;

import org.spongepowered.asm.mixin.Final;
import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.Shadow;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Constant;
import org.spongepowered.asm.mixin.injection.ModifyConstant;
import org.spongepowered.asm.mixin.injection.ModifyVariable;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ChatModules;

import net.minecraft.client.gui.components.ChatComponent;
//? if >=26 {
import net.minecraft.client.multiplayer.chat.GuiMessage;
//?} else {
/*import net.minecraft.client.GuiMessage;
*///?}
import net.minecraft.network.chat.Component;

/** The Chat module: timestamps, [xN] for repeats, and a longer history. */
@Mixin(ChatComponent.class)
public abstract class ChatComponentMixin {
	@Shadow
	@Final
	private List<GuiMessage> allMessages;

	@Shadow
	protected abstract void refreshTrimmedMessages();

	//? if >=26 {
	@ModifyVariable(method = "addMessage(Lnet/minecraft/network/chat/Component;Lnet/minecraft/network/chat/MessageSignature;Lnet/minecraft/client/multiplayer/chat/GuiMessageSource;Lnet/minecraft/client/multiplayer/chat/GuiMessageTag;)V",
			at = @At("HEAD"), argsOnly = true)
	//?} else {
	/*@ModifyVariable(method = "addMessage(Lnet/minecraft/network/chat/Component;Lnet/minecraft/network/chat/MessageSignature;Lnet/minecraft/client/GuiMessageTag;)V",
			at = @At("HEAD"), argsOnly = true)
	*///?}
	private Component armstrong$decorate(Component message) {
		ChatModules.Chat chat = ArmstrongClient.active(ChatModules.Chat.class);
		if (chat == null) {
			return message;
		}
		// A repeat replaces the previous line, which then shows [x2], [x3]...
		if (chat.isRepeat(message) && !allMessages.isEmpty()
				&& allMessages.getFirst().content().getString().contains(message.getString())) {
			allMessages.removeFirst();
			refreshTrimmedMessages();
		}
		return chat.decorate(message);
	}

	@ModifyConstant(method = {"addMessageToQueue", "addMessageToDisplayQueue"}, constant = @Constant(intValue = 100))
	private int armstrong$history(int vanilla) {
		ChatModules.Chat chat = ArmstrongClient.active(ChatModules.Chat.class);
		return chat == null ? vanilla : chat.history();
	}
}
