// Package control 提供镜像控制（tap / swipe / text / key 等）的应用层。
package control

import (
	"context"
	"errors"

	appinstance "assassin-android-controller/internal/application/instance"
	infrascrcpy "assassin-android-controller/internal/infrastructure/scrcpy"
)

var (
	ErrNoSerial = errors.New("control: instance not running")
)

// ADBControl 抽象 adb.Client 中控制相关的方法（兼容老接口，承担 swipe 等高级动作）。
type ADBControl interface {
	InputTap(ctx context.Context, serial string, x, y int) error
	InputSwipe(ctx context.Context, serial string, x1, y1, x2, y2, durationMs int) error
	InputText(ctx context.Context, serial, text string) error
	InputKeyEvent(ctx context.Context, serial string, code int) error
}

// ScrcpyControl 抽象向 scrcpy 控制通道发送字节。SessionManager 实现该接口。
type ScrcpyControl interface {
	SendControl(ctx context.Context, serial string, payload []byte) error
	Dimensions(serial string) (w, h uint16, ok bool)
}

// Service 表示控制应用服务，所有写操作都先取实例锁。
type Service struct {
	instances *appinstance.InstanceService
	adb       ADBControl
	scrcpy    ScrcpyControl // 可为 nil；nil 时新接口返回 ErrNoSerial。
}

func New(instances *appinstance.InstanceService, adb ADBControl) *Service {
	return &Service{instances: instances, adb: adb}
}

// WithScrcpy 注入 scrcpy 控制通道，启用低延迟多指 / 系统键接口。
func (s *Service) WithScrcpy(sc ScrcpyControl) *Service {
	s.scrcpy = sc
	return s
}

// scrcpyFingerPointerID 是 scrcpy INJECT_TOUCH_EVENT 里"虚拟手指"的特殊 pointer id。
// 与 scrcpy 上游 ScrcpyPointerId.Finger 对应（uint64 max-1）。
const scrcpyFingerPointerID uint64 = 0xFFFFFFFFFFFFFFFE

// Tap 走 scrcpy INJECT_TOUCH_EVENT（Down + Up）。
//
// 消息里带 screenW/screenH（= 当前 scrcpy 镜像分辨率），scrcpy server 内部按
// actualX = x * deviceW / screenW 把镜像坐标映射回设备原生坐标。所以前端从 canvas
// 像素拿到的镜像分辨率坐标可以直接传过来，端到端不需要任何缩放计算。
//
// 不再退回 `adb shell input tap`：那条老路径要的是设备原生分辨率坐标，
// 前端送的是镜像缩放后的坐标，落点会错位（max_size=720 时坐标会偏 ~0.56 倍）。
// 隐式回退会让坐标 bug 不再炸出来，反而变成"看起来在工作但点击偏一点"的难发现 bug。
func (s *Service) Tap(ctx context.Context, userID, instanceID uint, x, y int) error {
	if err := s.Touch(ctx, userID, instanceID,
		infrascrcpy.ActionDown, scrcpyFingerPointerID, int32(x), int32(y), 1.0); err != nil {
		return err
	}
	return s.Touch(ctx, userID, instanceID,
		infrascrcpy.ActionUp, scrcpyFingerPointerID, int32(x), int32(y), 0)
}

func (s *Service) Swipe(ctx context.Context, userID, instanceID uint, x1, y1, x2, y2, durMs int) error {
	return s.run(ctx, userID, instanceID, func(serial string) error {
		return s.adb.InputSwipe(ctx, serial, x1, y1, x2, y2, durMs)
	})
}

func (s *Service) Text(ctx context.Context, userID, instanceID uint, text string) error {
	return s.run(ctx, userID, instanceID, func(serial string) error {
		return s.adb.InputText(ctx, serial, text)
	})
}

func (s *Service) Key(ctx context.Context, userID, instanceID uint, code int) error {
	return s.run(ctx, userID, instanceID, func(serial string) error {
		return s.adb.InputKeyEvent(ctx, serial, code)
	})
}

// Touch 通过 scrcpy 控制通道发送多指触摸事件。坐标必须是设备坐标。
//
// 调用方需要先有 video session（前端 attach mirror）才能取到 screen W/H；
// 没有 session 时返回 ErrNoSerial 让前端先建镜像。
func (s *Service) Touch(ctx context.Context, userID, instanceID uint,
	action infrascrcpy.Action, pointerID uint64, x, y int32, pressure float32) error {
	return s.runScrcpy(ctx, userID, instanceID, func(serial string) error {
		w, h, ok := s.scrcpy.Dimensions(serial)
		if !ok {
			return ErrNoSerial
		}
		payload := infrascrcpy.EncodeTouch(action, pointerID, x, y, w, h, pressure, 0, 1)
		return s.scrcpy.SendControl(ctx, serial, payload)
	})
}

// ScrcpyText 走 scrcpy 控制通道注入文本。
func (s *Service) ScrcpyText(ctx context.Context, userID, instanceID uint, text string) error {
	return s.runScrcpy(ctx, userID, instanceID, func(serial string) error {
		return s.scrcpy.SendControl(ctx, serial, infrascrcpy.EncodeText(text))
	})
}

// BackOrScreenOn 触发 BACK_OR_SCREEN_ON：屏幕灭则点亮，否则等价返回键。
func (s *Service) BackOrScreenOn(ctx context.Context, userID, instanceID uint, action infrascrcpy.Action) error {
	return s.runScrcpy(ctx, userID, instanceID, func(serial string) error {
		return s.scrcpy.SendControl(ctx, serial, infrascrcpy.EncodeBackOrScreenOn(action))
	})
}

// ExpandNotificationPanel 展开通知栏。
func (s *Service) ExpandNotificationPanel(ctx context.Context, userID, instanceID uint) error {
	return s.runScrcpy(ctx, userID, instanceID, func(serial string) error {
		return s.scrcpy.SendControl(ctx, serial, infrascrcpy.EncodeExpandNotificationPanel())
	})
}

// CollapsePanels 收起通知栏 / 快捷开关。
func (s *Service) CollapsePanels(ctx context.Context, userID, instanceID uint) error {
	return s.runScrcpy(ctx, userID, instanceID, func(serial string) error {
		return s.scrcpy.SendControl(ctx, serial, infrascrcpy.EncodeCollapsePanels())
	})
}

func (s *Service) run(ctx context.Context, userID, instanceID uint, fn func(serial string) error) error {
	inst, err := s.instances.LookupOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return err
	}
	if inst.Serial == "" {
		return ErrNoSerial
	}
	unlock := s.instances.LockInstance(instanceID)
	defer unlock()
	return fn(inst.Serial)
}

// runScrcpy 与 run 共用模板，但额外做 scrcpy nil 检查。
func (s *Service) runScrcpy(ctx context.Context, userID, instanceID uint, fn func(serial string) error) error {
	if s.scrcpy == nil {
		return ErrNoSerial
	}
	return s.run(ctx, userID, instanceID, fn)
}
