// Package bench sets up and reads `armstrong bench` runs: fixed game options,
// so runs compare fairly, and the results the client mod writes. The mod
// itself creates the world (client-mod/.../bench/Bench.java).
package bench

import (
	"os"
	"path/filepath"
)

// WorldName is the save folder of the bench world. Its seed is fixed in the
// mod, but terrain differs between Minecraft versions, so only compare runs
// of the same version.
const WorldName = "armstrong-bench"

// ResetWorld deletes the last run's world, so every run generates the same
// terrain from scratch.
func ResetWorld(gameDir string) error {
	return os.RemoveAll(filepath.Join(gameDir, "saves", WorldName))
}
