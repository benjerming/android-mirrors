package scrcpy

import "time"

// RestartPolicyOpts 描述指数退避 + 熔断的参数。零值走默认。
type RestartPolicyOpts struct {
	BaseDelay   time.Duration // 默认 1s
	MaxDelay    time.Duration // 默认 16s
	MaxAttempts int           // 默认 5；超过后 NextDelay 返回 open=true
}

// RestartPolicy 把"下一次该等多久 / 该不该熔断"的决策抽出来便于测试和指标。
// 非 goroutine-safe；调用方（supervisor）保证串行调用。
type RestartPolicy struct {
	opts     RestartPolicyOpts
	attempts int
}

func NewRestartPolicy(opts RestartPolicyOpts) *RestartPolicy {
	if opts.BaseDelay <= 0 {
		opts.BaseDelay = time.Second
	}
	if opts.MaxDelay < opts.BaseDelay {
		opts.MaxDelay = 16 * time.Second
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 5
	}
	return &RestartPolicy{opts: opts}
}

// NextDelay 返回这一次应等待的延迟；open=true 表示已超过最大尝试次数，
// supervisor 应停止重启并让进 Failed 状态。
func (p *RestartPolicy) NextDelay() (time.Duration, bool) {
	if p.attempts >= p.opts.MaxAttempts {
		return 0, true
	}
	p.attempts++
	d := p.opts.BaseDelay << (p.attempts - 1)
	if d > p.opts.MaxDelay {
		d = p.opts.MaxDelay
	}
	return d, false
}

// Reset 清零 attempts；下一次 NextDelay 从 BaseDelay 开始。
func (p *RestartPolicy) Reset() {
	p.attempts = 0
}

// Attempts 返回已累积的尝试次数（指标用）。
func (p *RestartPolicy) Attempts() int { return p.attempts }
