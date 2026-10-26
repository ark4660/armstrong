package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;

/**
 * Time Changer: the client's world clock reports the chosen time of day.
 * 26.x has world clocks; before that, the client level's day time.
 */
//? if >=26 {
@Mixin(net.minecraft.client.ClientClockManager.ClientClockInstance.class)
//?} else {
/*@Mixin(net.minecraft.client.multiplayer.ClientLevel.ClientLevelData.class)
*///?}
public class ClockMixin {
	//? if >=26 {
	@Inject(method = "totalTicks", at = @At("RETURN"), cancellable = true)
	//?} else {
	/*@Inject(method = "getDayTime", at = @At("RETURN"), cancellable = true)
	*///?}
	private void armstrong$time(CallbackInfoReturnable<Long> cir) {
		ViewModules.TimeChanger tc = ArmstrongClient.active(ViewModules.TimeChanger.class);
		if (tc != null) {
			long total = cir.getReturnValueJ();
			cir.setReturnValue(Math.floorDiv(total, 24000L) * 24000L + tc.ticks());
		}
	}

	//? if >=26 {
	@Inject(method = "partialTick", at = @At("RETURN"), cancellable = true)
	private void armstrong$still(CallbackInfoReturnable<Float> cir) {
		if (ArmstrongClient.active(ViewModules.TimeChanger.class) != null) {
			cir.setReturnValue(0f); // a fixed time doesn't creep between ticks
		}
	}
	//?}
}
