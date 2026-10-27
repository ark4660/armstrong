package com.ark4660.armstrong.bench;

import java.lang.management.GarbageCollectorMXBean;
import java.lang.management.ManagementFactory;
import java.lang.management.MemoryPoolMXBean;
import java.lang.management.MemoryType;
import java.lang.management.MemoryUsage;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import com.google.gson.GsonBuilder;

import net.fabricmc.loader.api.FabricLoader;
import net.fabricmc.loader.api.ModContainer;

/**
 * The Minecraft-free half of `armstrong bench`: frame times, garbage
 * collection and render-thread allocation over a measured window, written as
 * JSON for the launcher. Each version's Bench drives it (when the window
 * starts, one call per frame) and adds what only the game knows.
 */
public final class BenchRecorder {
	/** Warm-up and measured seconds, the scene and the result file, from -Darmstrong.bench.* properties. */
	public final String scene = System.getProperty("armstrong.bench.scene", "static");
	public final long warmupNs = seconds("armstrong.bench.warmup", 10);
	public final long durationNs = seconds("armstrong.bench.duration", 30);
	private final Path out = Path.of(System.getProperty("armstrong.bench.out", "bench-result.json"));

	private long[] frames = new long[1 << 16];
	private int n, unfocused;
	private long last, windowStart, windowEnd, allocStart;
	private long[] gcCountStart, gcTimeStart;
	private long readyMs = -1, inWorldMs = -1;
	private boolean finished;

	/** Whether benchmarking was asked for on the command line. */
	public static boolean requested() {
		return System.getProperty("armstrong.bench.scene") != null;
	}

	private static long seconds(String property, long fallback) {
		return (long) (Double.parseDouble(System.getProperty(property, Long.toString(fallback))) * 1e9);
	}

	private static long sinceJvmStart() {
		return System.currentTimeMillis() - ManagementFactory.getRuntimeMXBean().getStartTime();
	}

	/** The game finished loading (title screen reachable). */
	public void markReady() {
		if (readyMs < 0) {
			readyMs = sinceJvmStart();
		}
	}

	/** The player is in the world: the warm-up starts now. */
	public void markInWorld() {
		if (inWorldMs < 0) {
			inWorldMs = sinceJvmStart();
			windowStart = System.nanoTime() + warmupNs;
			windowEnd = windowStart + durationNs;
		}
	}

	public boolean inWorld() {
		return inWorldMs >= 0;
	}

	/** Seconds since the player arrived, for scenes that move over time. */
	public double secondsInWorld() {
		return inWorldMs < 0 ? 0 : (sinceJvmStart() - inWorldMs) / 1000.0;
	}

	/**
	 * Call once per rendered frame on the render thread, saying whether the
	 * window has focus (frame limiters slow down unfocused windows). Returns
	 * true once the measured window is over (exactly once): call finish then.
	 */
	public boolean frame(boolean focused) {
		long now = System.nanoTime();
		if (finished || inWorldMs < 0 || now < windowStart) {
			last = now;
			return false;
		}
		if (gcCountStart == null) {
			startWindow();
			last = now;
			return false;
		}
		if (n == frames.length) {
			frames = Arrays.copyOf(frames, n * 2);
		}
		frames[n++] = now - last;
		if (!focused) {
			unfocused++;
		}
		last = now;
		if (now >= windowEnd) {
			finished = true;
			return true;
		}
		return false;
	}

	private void startWindow() {
		List<GarbageCollectorMXBean> gcs = ManagementFactory.getGarbageCollectorMXBeans();
		gcCountStart = new long[gcs.size()];
		gcTimeStart = new long[gcs.size()];
		for (int i = 0; i < gcs.size(); i++) {
			gcCountStart[i] = gcs.get(i).getCollectionCount();
			gcTimeStart[i] = gcs.get(i).getCollectionTime();
		}
		allocStart = threadAllocated();
	}

	private static long threadAllocated() {
		if (ManagementFactory.getThreadMXBean() instanceof com.sun.management.ThreadMXBean t) {
			return t.getCurrentThreadAllocatedBytes();
		}
		return -1;
	}

	/**
	 * Writes the result. game holds what the version-specific side knows
	 * (Minecraft version, renderer, settings); it's merged in as is.
	 */
	public void finish(Map<String, Object> game) throws java.io.IOException {
		long alloc = threadAllocated();
		long[] f = Arrays.copyOf(frames, n);
		long total = 0;
		for (long x : f) {
			total += x;
		}
		Arrays.sort(f);
		double mean = total / (double) n;
		double var = 0;
		for (long x : f) {
			var += (x - mean) * (x - mean);
		}
		long median = f[n / 2];
		int stutters = 0;
		for (long x : f) {
			if (x > 2 * median && x > 4_000_000L) {
				stutters++;
			}
		}

		Map<String, Object> frame = new LinkedHashMap<>();
		frame.put("count", n);
		frame.put("avgFps", round(n / (total / 1e9)));
		frame.put("low1Fps", round(lowFps(f, 0.01)));
		frame.put("low01Fps", round(lowFps(f, 0.001)));
		frame.put("meanMs", round(mean / 1e6));
		frame.put("p50Ms", round(median / 1e6));
		frame.put("p90Ms", round(pct(f, 0.90) / 1e6));
		frame.put("p99Ms", round(pct(f, 0.99) / 1e6));
		frame.put("p999Ms", round(pct(f, 0.999) / 1e6));
		frame.put("maxMs", round(f[n - 1] / 1e6));
		frame.put("stdDevMs", round(Math.sqrt(var / n) / 1e6));
		frame.put("stutters", stutters);
		frame.put("unfocused", unfocused);
		if (alloc >= 0 && allocStart >= 0) {
			frame.put("allocKBPerFrame", round((alloc - allocStart) / 1024.0 / n));
		}

		List<Map<String, Object>> gc = new ArrayList<>();
		List<GarbageCollectorMXBean> gcs = ManagementFactory.getGarbageCollectorMXBeans();
		for (int i = 0; i < gcs.size() && i < gcCountStart.length; i++) {
			Map<String, Object> c = new LinkedHashMap<>();
			c.put("name", gcs.get(i).getName());
			c.put("count", gcs.get(i).getCollectionCount() - gcCountStart[i]);
			c.put("timeMs", gcs.get(i).getCollectionTime() - gcTimeStart[i]);
			gc.add(c);
		}

		long afterGc = 0;
		for (MemoryPoolMXBean pool : ManagementFactory.getMemoryPoolMXBeans()) {
			MemoryUsage u = pool.getType() == MemoryType.HEAP ? pool.getCollectionUsage() : null;
			if (u != null) {
				afterGc += u.getUsed();
			}
		}
		MemoryUsage heap = ManagementFactory.getMemoryMXBean().getHeapMemoryUsage();
		Map<String, Object> memory = new LinkedHashMap<>();
		memory.put("heapUsedMB", heap.getUsed() >> 20);
		memory.put("heapCommittedMB", heap.getCommitted() >> 20);
		memory.put("heapMaxMB", heap.getMax() >> 20);
		memory.put("heapAfterLastGcMB", afterGc >> 20);

		Map<String, Object> startup = new LinkedHashMap<>();
		startup.put("readyMs", readyMs);
		startup.put("inWorldMs", inWorldMs);

		Map<String, Object> jvm = new LinkedHashMap<>();
		jvm.put("version", System.getProperty("java.version"));
		jvm.put("vm", System.getProperty("java.vm.name"));
		jvm.put("args", ManagementFactory.getRuntimeMXBean().getInputArguments().stream()
				.filter(a -> !a.startsWith("-Dfabric.addMods") && !a.startsWith("-Djava.library.path")
						&& !a.startsWith("-Djna.") && !a.startsWith("-Dorg.lwjgl") && !a.startsWith("-Dio.netty"))
				.toList());
		jvm.put("cpus", Runtime.getRuntime().availableProcessors());

		Map<String, String> mods = new java.util.TreeMap<>();
		for (ModContainer m : FabricLoader.getInstance().getAllMods()) {
			String id = m.getMetadata().getId();
			if (m.getContainingMod().isEmpty() && !id.equals("java") && !id.equals("minecraft")) {
				mods.put(id, m.getMetadata().getVersion().getFriendlyString());
			}
		}

		Map<String, Object> result = new LinkedHashMap<>();
		result.put("format", 1);
		result.put("scene", scene);
		result.put("warmupS", warmupNs / 1e9);
		result.put("durationS", durationNs / 1e9);
		result.putAll(game);
		result.put("frames", frame);
		result.put("gc", gc);
		result.put("memory", memory);
		result.put("startup", startup);
		result.put("jvm", jvm);
		result.put("os", System.getProperty("os.name") + " " + System.getProperty("os.version") + " " + System.getProperty("os.arch"));
		result.put("mods", mods);
		Files.write(out, new GsonBuilder().setPrettyPrinting().create().toJson(result).getBytes(StandardCharsets.UTF_8));
	}

	/** Mean fps over the slowest share of frames (the "1% low"). */
	private static double lowFps(long[] sorted, double share) {
		int k = Math.max(1, (int) (sorted.length * share));
		long sum = 0;
		for (int i = sorted.length - k; i < sorted.length; i++) {
			sum += sorted[i];
		}
		return k / (sum / 1e9);
	}

	private static long pct(long[] sorted, double p) {
		return sorted[Math.min(sorted.length - 1, (int) (sorted.length * p))];
	}

	private static double round(double v) {
		return Math.round(v * 100) / 100.0;
	}
}
