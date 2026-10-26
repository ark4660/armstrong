package com.ark4660.armstrong.mixin;

import java.util.concurrent.ThreadLocalRandom;

import org.spongepowered.asm.mixin.Mixin;
import org.spongepowered.asm.mixin.injection.At;
import org.spongepowered.asm.mixin.injection.Inject;
import org.spongepowered.asm.mixin.injection.callback.CallbackInfo;

import com.ark4660.armstrong.ArmstrongClient;
import com.ark4660.armstrong.modules.ViewModules;

import net.minecraft.client.particle.Particle;
import net.minecraft.client.particle.ParticleManager;

/** The Particles module: drops a share of new particles. */
@Mixin(ParticleManager.class)
public class ParticleManagerMixin {
	@Inject(method = "addParticle(Lnet/minecraft/client/particle/Particle;)V", at = @At("HEAD"), cancellable = true)
	private void armstrong$fewer(Particle particle, CallbackInfo ci) {
		ViewModules.Particles p = ArmstrongClient.active(ViewModules.Particles.class);
		if (p != null && ThreadLocalRandom.current().nextDouble() >= p.keep()) {
			ci.cancel();
		}
	}
}
