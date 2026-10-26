package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;
import com.llamalad7.mixinextras.injector.ModifyExpressionValue;

import net.minecraft.client.world.ClientWorld;
import net.minecraft.world.World;

/**
 * Time and Weather Changer. World is shared with the integrated server in
 * singleplayer, so only the client's world is changed.
 */
@Mixin(World.class)
public class WorldMixin {
	@ModifyExpressionValue(method = "getSkyAngle", at = @At(value = "INVOKE", target = "Lnet/minecraft/world/level/LevelProperties;getTimeOfDay()J"))
	private long armstrong$time(long time) {
		ViewModules.TimeChanger tc = ArmstrongClient.active(ViewModules.TimeChanger.class);
		return tc != null && client() ? tc.ticks() : time;
	}

	@Inject(method = "getRainGradient", at = @At("RETURN"), cancellable = true)
	private void armstrong$rain(float delta, CallbackInfoReturnable<Float> cir) {
		ViewModules.WeatherChanger wc = ArmstrongClient.active(ViewModules.WeatherChanger.class);
		if (wc != null && client()) {
			cir.setReturnValue(wc.rain());
		}
	}

	@Inject(method = "getThunderGradient", at = @At("RETURN"), cancellable = true)
	private void armstrong$thunder(float delta, CallbackInfoReturnable<Float> cir) {
		ViewModules.WeatherChanger wc = ArmstrongClient.active(ViewModules.WeatherChanger.class);
		if (wc != null && client()) {
			cir.setReturnValue(wc.thunder());
		}
	}

	private boolean client() {
		return (Object) this instanceof ClientWorld;
	}
}
