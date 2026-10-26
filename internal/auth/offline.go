// Package auth handles player accounts: offline names and Microsoft login.
package auth

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"regexp"
)

var offlineName = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)

func ValidateOfflineName(name string) error {
	if !offlineName.MatchString(name) {
		return fmt.Errorf("offline name must be 3–16 characters of A–Z, a–z, 0–9 or _")
	}
	return nil
}

// OfflineUUID is the name-based (version 3) UUID Minecraft uses for offline
// players: MD5 of "OfflinePlayer:<name>". Returned without dashes.
func OfflineUUID(name string) string {
	sum := md5.Sum([]byte("OfflinePlayer:" + name))
	sum[6] = sum[6]&0x0f | 0x30 // version 3
	sum[8] = sum[8]&0x3f | 0x80 // IETF variant
	return hex.EncodeToString(sum[:])
}
