package com.ark4660.armstrong.mixin;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfoReturnable;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;

import net.minecraft.client.multiplayer.ClientLevel;
import net.minecraft.world.level.Level;

/** Weather Changer: the client's world reports the chosen weather (the server's stays real). */
@Mixin(Level.class)
public class LevelWeatherMixin {
	@Inject(method = "getRainLevel", at = @At("HEAD"), cancellable = true)
	private void armstrong$rain(float partialTick, CallbackInfoReturnable<Float> cir) {
		if ((Object) this instanceof ClientLevel) {
			ViewModules.WeatherChanger wc = ArmstrongClient.active(ViewModules.WeatherChanger.class);
			if (wc != null) {
				cir.setReturnValue(wc.rain());
			}
		}
	}

	@Inject(method = "getThunderLevel", at = @At("HEAD"), cancellable = true)
	private void armstrong$thunder(float partialTick, CallbackInfoReturnable<Float> cir) {
		if ((Object) this instanceof ClientLevel) {
			ViewModules.WeatherChanger wc = ArmstrongClient.active(ViewModules.WeatherChanger.class);
			if (wc != null) {
				cir.setReturnValue(wc.thunder());
			}
		}
	}
}
