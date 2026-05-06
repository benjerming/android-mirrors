package scrcpy_test

import (
	"bytes"
	"testing"

	"assassin-android-controller/internal/infrastructure/scrcpy"
)

func TestControlCodec_InjectKeycode(t *testing.T) {
	got := scrcpy.EncodeKeycode(scrcpy.ActionDown, 4 /*KEYCODE_BACK*/, 0, 0)
	want := []byte{0x00, 0x00, 0, 0, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

func TestControlCodec_InjectText(t *testing.T) {
	got := scrcpy.EncodeText("hi")
	want := []byte{0x01, 0, 0, 0, 2, 'h', 'i'}
	if !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

func TestControlCodec_InjectTouch(t *testing.T) {
	got := scrcpy.EncodeTouch(scrcpy.ActionDown, 0, 100, 200, 1080, 1920, 1.0, 0, 1)
	if len(got) != 32 {
		t.Fatalf("touch packet must be 32 bytes, got %d", len(got))
	}
	if got[0] != 0x02 {
		t.Errorf("type byte must be 2, got %x", got[0])
	}
	if got[1] != 0x00 {
		t.Errorf("action byte must be 0 (down), got %x", got[1])
	}
	for i := 2; i < 10; i++ {
		if got[i] != 0 {
			t.Errorf("pointerId byte %d must be 0", i)
		}
	}
	if !bytes.Equal(got[10:14], []byte{0, 0, 0, 100}) {
		t.Errorf("x bytes wrong: %x", got[10:14])
	}
	if !bytes.Equal(got[14:18], []byte{0, 0, 0, 200}) {
		t.Errorf("y bytes wrong: %x", got[14:18])
	}
	if !bytes.Equal(got[18:20], []byte{0x04, 0x38}) { // 1080 = 0x0438
		t.Errorf("screenW wrong: %x", got[18:20])
	}
	if !bytes.Equal(got[20:22], []byte{0x07, 0x80}) { // 1920 = 0x0780
		t.Errorf("screenH wrong: %x", got[20:22])
	}
	if !bytes.Equal(got[22:24], []byte{0xFF, 0xFF}) { // pressure 1.0 -> 0xFFFF
		t.Errorf("pressure bytes wrong: %x", got[22:24])
	}
}

func TestControlCodec_BackOrScreenOn(t *testing.T) {
	got := scrcpy.EncodeBackOrScreenOn(scrcpy.ActionDown)
	want := []byte{0x04, 0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

func TestControlCodec_ExpandAndCollapse(t *testing.T) {
	if !bytes.Equal(scrcpy.EncodeExpandNotificationPanel(), []byte{0x05}) {
		t.Errorf("expand panel wrong")
	}
	if !bytes.Equal(scrcpy.EncodeCollapsePanels(), []byte{0x07}) {
		t.Errorf("collapse panels wrong")
	}
}
