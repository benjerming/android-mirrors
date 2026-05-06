package scrcpy_test

import (
	"bytes"
	"testing"

	"assassin-android-controller/internal/infrastructure/scrcpy"
)

// minimal SPS/PPS/IDR for a real 1280x720 baseline stream (truncated payloads OK for header detection).
var (
	testSPS = []byte{0x00, 0x00, 0x00, 0x01,
		0x67, 0x42, 0xc0, 0x1f, 0xd9, 0x00, 0x50, 0x05, 0xbb, 0x01, 0x6c, 0x80, 0x00, 0x00, 0x03,
		0x00, 0x80, 0x00, 0x00, 0x1e, 0x07, 0x8c, 0x18, 0xcb}
	testPPS = []byte{0x00, 0x00, 0x00, 0x01, 0x68, 0xce, 0x06, 0xe2}
	testIDR = []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84, 0x00, 0x00, 0x00, 0x01}
)

func TestFmp4_FirstFrameProducesInitAndFragment(t *testing.T) {
	mux := scrcpy.NewFmp4Muxer()
	frame := bytes.Join([][]byte{testSPS, testPPS, testIDR}, nil)
	init, frag, err := mux.WriteFrame(frame, 0)
	if err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	if len(init) == 0 {
		t.Errorf("init segment must not be empty on first frame")
	}
	if len(frag) == 0 {
		t.Errorf("fragment must not be empty")
	}
	if !bytes.Contains(init, []byte("ftyp")) || !bytes.Contains(init, []byte("moov")) {
		t.Errorf("init segment missing ftyp/moov")
	}
	if !bytes.Contains(frag, []byte("moof")) || !bytes.Contains(frag, []byte("mdat")) {
		t.Errorf("fragment missing moof/mdat")
	}
}

func TestFmp4_SubsequentFrameNoInit(t *testing.T) {
	mux := scrcpy.NewFmp4Muxer()
	first := bytes.Join([][]byte{testSPS, testPPS, testIDR}, nil)
	if _, _, err := mux.WriteFrame(first, 0); err != nil {
		t.Fatalf("first WriteFrame: %v", err)
	}
	pframe := []byte{0x00, 0x00, 0x00, 0x01, 0x41, 0xe0, 0x00}
	init, frag, err := mux.WriteFrame(pframe, 33000)
	if err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	if len(init) != 0 {
		t.Errorf("init segment should be nil after first frame, got %d bytes", len(init))
	}
	if len(frag) == 0 {
		t.Errorf("fragment must not be empty")
	}
}

func TestFmp4_RejectsFrameWithoutSpsPps(t *testing.T) {
	mux := scrcpy.NewFmp4Muxer()
	_, _, err := mux.WriteFrame(testIDR, 0)
	if err == nil {
		t.Errorf("expect error when first frame lacks SPS/PPS")
	}
}
