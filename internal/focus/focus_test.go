package focus

import "testing"

func TestEncodeDecodeConfig_RoundTrip(t *testing.T) {
	c := Config{BlockedApps: []string{"Steam.exe", "Discord.exe"}, BlockedWebsites: []string{"youtube.com"}, SuppressNotifications: true}
	encoded, err := EncodeConfig(c)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeConfig(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.BlockedApps) != 2 || decoded.BlockedApps[0] != "Steam.exe" {
		t.Fatalf("blocked apps mismatch: %+v", decoded)
	}
	if !decoded.SuppressNotifications {
		t.Fatal("expected SuppressNotifications to round-trip as true")
	}
}

func TestDecodeConfig_EmptyIsZeroValueNotError(t *testing.T) {
	c, err := DecodeConfig("")
	if err != nil {
		t.Fatalf("empty metadata must not be an error - a focus session with no blocking configured is valid, got: %v", err)
	}
	if len(c.BlockedApps) != 0 || c.SuppressNotifications {
		t.Fatalf("expected a zero Config, got %+v", c)
	}
}

func TestDecodeConfig_MalformedIsError(t *testing.T) {
	if _, err := DecodeConfig("not json"); err == nil {
		t.Fatal("expected an error for malformed metadata")
	}
}

func TestIsBlockedApp_CaseInsensitiveExactMatch(t *testing.T) {
	blocked := []string{"Steam.exe", "Discord.exe"}
	if !IsBlockedApp("steam.exe", blocked) {
		t.Fatal("expected a case-insensitive match")
	}
	if !IsBlockedApp("STEAM.EXE", blocked) {
		t.Fatal("expected a case-insensitive match")
	}
	if IsBlockedApp("Steam", blocked) {
		t.Fatal("must not partial-match - 'Steam' alone should not match 'Steam.exe'")
	}
	if IsBlockedApp("NotepadPlusPlus.exe", blocked) {
		t.Fatal("must not match an unrelated process")
	}
}

func TestIsBlockedApp_EmptyList(t *testing.T) {
	if IsBlockedApp("Anything.exe", nil) {
		t.Fatal("an empty blocklist must never match anything")
	}
}

func TestQuality_NoTrackedTimeYet(t *testing.T) {
	_, ok := Quality(0, 0)
	if ok {
		t.Fatal("zero tracked time must report ok=false, not a misleading percentage")
	}
}

func TestQuality_PerfectFocus(t *testing.T) {
	p, ok := Quality(1500, 1500)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if p != 100 {
		t.Fatalf("got %v, want 100", p)
	}
}

func TestQuality_PartialFocus(t *testing.T) {
	// 23 of 25 minutes on-target - matches spec section 20's own
	// "Focus quality: 92%" example almost exactly (23/25 = 92%).
	p, ok := Quality(23*60, 25*60)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if p != 92 {
		t.Fatalf("got %v, want 92", p)
	}
}

func TestQuality_ZeroOnTarget(t *testing.T) {
	p, ok := Quality(0, 900)
	if !ok {
		t.Fatal("expected ok=true (there IS tracked time, just none of it on-target)")
	}
	if p != 0 {
		t.Fatalf("got %v, want 0", p)
	}
}

func TestQuality_ClampsAboveHundred(t *testing.T) {
	// Shouldn't be reachable given callers only ever pass onTarget as a
	// subset of totalTracked, but a corrupted/manually-edited row must
	// never be able to display an impossible >100% quality.
	p, ok := Quality(2000, 1000)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if p != 100 {
		t.Fatalf("got %v, want clamped to 100", p)
	}
}
