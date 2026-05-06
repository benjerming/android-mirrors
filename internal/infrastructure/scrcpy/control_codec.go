package scrcpy

import "encoding/binary"

// scrcpy 控制消息类型常量（参考 scrcpy 3.x 协议）。
const (
	TypeInjectKeycode  byte = 0
	TypeInjectText     byte = 1
	TypeInjectTouch    byte = 2
	TypeBackOrScreenOn byte = 4
	TypeExpandNotif    byte = 5
	TypeCollapsePanels byte = 7
)

// Action 表示按键 / 触摸事件的动作。
type Action byte

const (
	ActionDown Action = 0
	ActionUp   Action = 1
	ActionMove Action = 2
)

// EncodeKeycode 打包 INJECT_KEYCODE 消息。所有 multi-byte 字段大端序。
//
// 字节布局（共 14B）：type(1) + action(1) + keycode(4) + repeat(4) + metaState(4)。
func EncodeKeycode(action Action, keycode int32, repeat int32, metaState int32) []byte {
	b := make([]byte, 14)
	b[0] = TypeInjectKeycode
	b[1] = byte(action)
	binary.BigEndian.PutUint32(b[2:6], uint32(keycode))
	binary.BigEndian.PutUint32(b[6:10], uint32(repeat))
	binary.BigEndian.PutUint32(b[10:14], uint32(metaState))
	return b
}

// EncodeText 打包 INJECT_TEXT。字节布局：type(1) + len(4) + utf8 bytes。
func EncodeText(text string) []byte {
	data := []byte(text)
	b := make([]byte, 5+len(data))
	b[0] = TypeInjectText
	binary.BigEndian.PutUint32(b[1:5], uint32(len(data)))
	copy(b[5:], data)
	return b
}

// EncodeTouch 打包 INJECT_TOUCH_EVENT（共 32B）：
// type(1) + action(1) + pointerId(8) + x(4) + y(4) + screenW(2) + screenH(2)
// + pressure(2 u16fixed) + actionButton(4) + buttons(4)。
//
// pressure 是 [0,1] 浮点，编码为 u16 fixed-point（0..0xFFFF）。
func EncodeTouch(action Action, pointerID uint64, x, y int32, screenW, screenH uint16,
	pressure float32, actionButton, buttons int32) []byte {
	b := make([]byte, 32)
	b[0] = TypeInjectTouch
	b[1] = byte(action)
	binary.BigEndian.PutUint64(b[2:10], pointerID)
	binary.BigEndian.PutUint32(b[10:14], uint32(x))
	binary.BigEndian.PutUint32(b[14:18], uint32(y))
	binary.BigEndian.PutUint16(b[18:20], screenW)
	binary.BigEndian.PutUint16(b[20:22], screenH)
	if pressure < 0 {
		pressure = 0
	}
	if pressure > 1 {
		pressure = 1
	}
	binary.BigEndian.PutUint16(b[22:24], uint16(pressure*0xFFFF))
	binary.BigEndian.PutUint32(b[24:28], uint32(actionButton))
	binary.BigEndian.PutUint32(b[28:32], uint32(buttons))
	return b
}

// EncodeBackOrScreenOn 打包 BACK_OR_SCREEN_ON：type(1) + action(1)。
//
// down 时如果屏幕灭则点亮屏幕，否则模拟返回键；up 不触发任何效果。
func EncodeBackOrScreenOn(action Action) []byte {
	return []byte{TypeBackOrScreenOn, byte(action)}
}

// EncodeExpandNotificationPanel 展开通知栏。
func EncodeExpandNotificationPanel() []byte { return []byte{TypeExpandNotif} }

// EncodeCollapsePanels 收起通知栏 / 快捷开关。
func EncodeCollapsePanels() []byte { return []byte{TypeCollapsePanels} }
