package httpapi

import (
	"testing"

	infrascrcpy "assassin-android-controller/internal/infrastructure/scrcpy"
)

func TestParseMirrorControlFrame_ValidTouchDown(t *testing.T) {
	got, err := parseMirrorControlFrame([]byte(`{"type":"touch","action":"down","x":100,"y":200,"pressure":1}`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Type != "touch" || got.Action != infrascrcpy.ActionDown {
		t.Fatalf("bad parse: %+v", got)
	}
	if got.X != 100 || got.Y != 200 || got.Pressure != 1.0 {
		t.Fatalf("bad coords: %+v", got)
	}
}

func TestParseMirrorControlFrame_ValidTouchMoveUp(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		action infrascrcpy.Action
	}{
		{`{"type":"touch","action":"move","x":1,"y":2,"pressure":1}`, infrascrcpy.ActionMove},
		{`{"type":"touch","action":"up","x":1,"y":2,"pressure":0}`, infrascrcpy.ActionUp},
	} {
		got, err := parseMirrorControlFrame([]byte(tc.raw))
		if err != nil {
			t.Fatalf("%s: %v", tc.raw, err)
		}
		if got.Action != tc.action {
			t.Fatalf("%s: action mismatch", tc.raw)
		}
	}
}

func TestParseMirrorControlFrame_InvalidJSON(t *testing.T) {
	if _, err := parseMirrorControlFrame([]byte(`{not json`)); err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestParseMirrorControlFrame_UnknownType(t *testing.T) {
	if _, err := parseMirrorControlFrame([]byte(`{"type":"wheel","action":"down"}`)); err == nil {
		t.Fatal("want error for unknown type")
	}
}

func TestParseMirrorControlFrame_UnknownAction(t *testing.T) {
	if _, err := parseMirrorControlFrame([]byte(`{"type":"touch","action":"hover"}`)); err == nil {
		t.Fatal("want error for unknown action")
	}
}

func TestParseMirrorControlFrame_PressureClamped(t *testing.T) {
	got, _ := parseMirrorControlFrame([]byte(`{"type":"touch","action":"down","x":1,"y":2,"pressure":5}`))
	if got.Pressure != 1.0 {
		t.Fatalf("pressure clamp failed: %v", got.Pressure)
	}
	got, _ = parseMirrorControlFrame([]byte(`{"type":"touch","action":"down","x":1,"y":2,"pressure":-1}`))
	if got.Pressure != 0.0 {
		t.Fatalf("pressure clamp failed: %v", got.Pressure)
	}
}
