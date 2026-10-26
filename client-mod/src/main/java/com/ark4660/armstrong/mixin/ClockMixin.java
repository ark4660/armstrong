package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;

import net.minecraft.client.ClientClockManager;

/** Time Changer: the client's world clocks report the chosen time of day. */
@Mixin(ClientClockManager.ClientClockInstance.class)
public class ClockMixin {
	@Inject(method = "totalTicks", at = @At("RETURN"), cancellable = true)
	private void armstrong$time(CallbackInfoReturnable<Long> cir) {
		ViewModules.TimeChanger tc = ArmstrongClient.active(ViewModules.TimeChanger.class);
		if (tc != null) {
			long total = cir.getReturnValueJ();
			cir.setReturnValue(Math.floorDiv(total, 24000L) * 24000L + tc.ticks());
		}
	}

	@Inject(method = "partialTick", at = @At("RETURN"), cancellable = true)
	private void armstrong$still(CallbackInfoReturnable<Float> cir) {
		if (ArmstrongClient.active(ViewModules.TimeChanger.class) != null) {
			cir.setReturnValue(0f); // a fixed time doesn't creep between ticks
		}
	}
}
