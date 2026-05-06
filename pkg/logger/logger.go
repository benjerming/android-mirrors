// Package logger 封装项目用的全局日志构造逻辑。
//
// 当前实现基于标准库 log/slog：开发环境用更易读的 Text handler，其它环境用结构化 JSON。
// 之所以放在 pkg 而不是 internal，是预留给未来可能的子命令 / 工具复用同一套日志风格。
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// requestIDCtxKey 是把 request_id 存到 context.Context 的私有键。
//
// 用未导出的类型避免和别处的 context key 撞 key；导出的入口是 WithRequestID / RequestIDFromContext。
type requestIDCtxKey struct{}

// requestIDLogAttr 是请求 ID 在日志里出现时使用的字段名，前后端约定保持一致方便检索。
const requestIDLogAttr = "request_id"

// WithRequestID 用来把请求 ID 塞进 context，让 service / repository 等下游层
// 不必修改函数签名就能拿到关联 ID。中间件在请求进来时调一次即可。
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDCtxKey{}, id)
}

// RequestIDFromContext 用来从 context 取出请求 ID；没有则返回空串。
// 业务代码一般不需要直接调它——ContextHandler 会自动把 ID 注入日志。
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// contextHandler 包装一个底层 slog.Handler，在每条日志写出前自动从 ctx 取 request_id 加进去。
//
// 选择"包装 handler"而不是"业务代码每次手动 With"，是为了让业务调用 slog.InfoContext 即可，
// 不需要每个函数先 logger.FromContext(ctx) 再用——少一步就少一处忘记带 ID 的回归。
type contextHandler struct {
	slog.Handler
}

// Handle 在交给底层 handler 之前，把 ctx 里的 request_id 作为属性补到 record 上。
func (h contextHandler) Handle(ctx context.Context, record slog.Record) error {
	if id := RequestIDFromContext(ctx); id != "" {
		record.AddAttrs(slog.String(requestIDLogAttr, id))
	}
	return h.Handler.Handle(ctx, record)
}

// WithAttrs / WithGroup 透传给底层 handler，并保持外层包装，
// 这样调用方拿到 logger.With(...) 之后仍然会带上 ctx 注入逻辑。
func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{Handler: h.Handler.WithGroup(name)}
}

// New 用来创建全局日志实例；开发环境走可读性更强的输出，其他环境走结构化输出。
//
// 返回的 logger 已经包了 ContextHandler，调用 slog.InfoContext(ctx, ...) 时会自动带上 request_id。
func New(env string) (*slog.Logger, error) {
	var base slog.Handler
	if strings.EqualFold(env, "dev") {
		base = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	} else {
		base = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	return slog.New(contextHandler{Handler: base}), nil
}
