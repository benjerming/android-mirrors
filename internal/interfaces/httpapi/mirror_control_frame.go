package httpapi

import (
	"encoding/json"
	"errors"

	infrascrcpy "assassin-android-controller/internal/infrastructure/scrcpy"
)

// mirrorControlFrame 是镜像 WS 上"客户端 → 服务端"控制消息解析后的表示。
// 协议见 docs/superpowers/specs/2026-05-06-mirror-swipe-design.md §2.3。
type mirrorControlFrame struct {
	Type     string
	Action   infrascrcpy.Action
	X        int32
	Y        int32
	Pressure float32
}

type mirrorControlFrameWire struct {
	Type     string  `json:"type"`
	Action   string  `json:"action"`
	X        int32   `json:"x"`
	Y        int32   `json:"y"`
	Pressure float32 `json:"pressure"`
}

var (
	errUnknownControlType   = errors.New("unknown control type")
	errUnknownControlAction = errors.New("unknown control action")
)

func parseMirrorControlFrame(raw []byte) (mirrorControlFrame, error) {
	var w mirrorControlFrameWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return mirrorControlFrame{}, err
	}
	if w.Type != "touch" {
		return mirrorControlFrame{}, errUnknownControlType
	}
	act, ok := parseAction(w.Action)
	if !ok {
		return mirrorControlFrame{}, errUnknownControlAction
	}
	p := w.Pressure
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	return mirrorControlFrame{
		Type:     w.Type,
		Action:   act,
		X:        w.X,
		Y:        w.Y,
		Pressure: p,
	}, nil
}
