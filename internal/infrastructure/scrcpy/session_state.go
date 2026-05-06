package scrcpy

// SessionState 描述 Session 的显式生命周期。
//
// 转换图（单向）：
//
//	Starting --(pump 起来)--> Running --(Close 开始)--> Closing --(pump 退出)--> Dead
//	Starting -------------(OpenSession 后立即 Close)-------------------------> Dead
//	Running --------------(pump 自然退出，无 Close)-------------------------> Dead
//
// 任何向后转换都是 bug；CanTransition 用于 Session.transitionTo 中防御。
type SessionState int32

const (
	StateStarting SessionState = iota
	StateRunning
	StateClosing
	StateDead
)

func (s SessionState) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateClosing:
		return "closing"
	case StateDead:
		return "dead"
	default:
		return "unknown"
	}
}

// CanTransition 是否允许从 s 转到 next。
func (s SessionState) CanTransition(next SessionState) bool {
	switch s {
	case StateStarting:
		return next == StateRunning || next == StateDead
	case StateRunning:
		return next == StateClosing || next == StateDead
	case StateClosing:
		return next == StateDead
	default:
		return false
	}
}
