package assets

import "testing"

func TestLoadTitleBinary(t *testing.T) {
	tb, err := LoadTitleBinary()
	if err != nil {
		t.Fatalf("LoadTitleBinary failed: %v", err)
	}
	if len(tb) != 28011 {
		t.Fatalf("expected embedded binary length 28011, got %d", len(tb))
	}
	// Verify expected binary header bytes
	if tb[0] != 0x4C || tb[1] != 0x00 || tb[2] != 0x94 {
		t.Fatalf("unexpected entry bytes: %02X %02X %02X", tb[0], tb[1], tb[2])
	}
}
