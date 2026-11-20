package ui

import "testing"

func TestMonochromeShaderCompiles(t *testing.T) {
	sh, err := newMonochromeShader()
	if err != nil {
		t.Fatalf("newMonochromeShader failed: %v", err)
	}
	if sh == nil {
		t.Fatalf("expected non-nil shader")
	}
}
