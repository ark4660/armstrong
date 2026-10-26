package auth

import "testing"

func TestOfflineUUID(t *testing.T) {
	// The well-known offline UUID for "Notch" (b50ad385-829d-3141-a216-7e7d7539ba7f).
	if got, want := OfflineUUID("Notch"), "b50ad385829d3141a2167e7d7539ba7f"; got != want {
		t.Errorf("OfflineUUID(Notch) = %s, want %s", got, want)
	}
}

func TestValidateOfflineName(t *testing.T) {
	for _, ok := range []string{"Steve", "abc", "Player_123", "abcdefghijklmnop"} {
		if err := ValidateOfflineName(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"ab", "abcdefghijklmnopq", "has space", "dash-name", ""} {
		if ValidateOfflineName(bad) == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}
