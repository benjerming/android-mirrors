package bootstrap

import (
	"context"
	"testing"
	"time"

	"assassin-android-controller/internal/config"

	"github.com/gin-gonic/gin"
)

// TestAppNewHTTPServerUsesConfiguredTimeouts 用来确认生产启动时会创建带超时保护的 HTTP 服务。
func TestAppNewHTTPServerUsesConfiguredTimeouts(t *testing.T) {
	app := &App{
		Config: &config.Config{
			App: config.AppConfig{
				HTTPPort: 18080,
			},
		},
		Router: gin.New(),
	}

	server := app.NewHTTPServer()

	if server.Addr != ":18080" {
		t.Fatalf("expected addr :18080, got %q", server.Addr)
	}
	if server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("expected read header timeout 5s, got %s", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != 15*time.Second {
		t.Fatalf("expected read timeout 15s, got %s", server.ReadTimeout)
	}
	if server.WriteTimeout != 15*time.Second {
		t.Fatalf("expected write timeout 15s, got %s", server.WriteTimeout)
	}
	if server.IdleTimeout != 60*time.Second {
		t.Fatalf("expected idle timeout 60s, got %s", server.IdleTimeout)
	}
}

// TestWatchShutdownCallsShutdownAfterContextCancel 用来确认收到取消信号后，会触发标准库服务的优雅停机。
func TestWatchShutdownCallsShutdownAfterContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := &fakeShutdowner{
		shutdownCalled: make(chan struct{}, 1),
	}

	stop := watchShutdown(ctx, server, nil)
	defer stop()

	cancel()

	select {
	case <-server.shutdownCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown was not called after context cancel")
	}
}

// fakeShutdowner 表示测试里的假服务，只记录有没有收到优雅停机调用。
type fakeShutdowner struct {
	shutdownCalled chan struct{} // shutdownCalled 表示一旦收到 Shutdown，就往这个通道发信号。
}

// Shutdown 用来模拟标准库 HTTP 服务的停机动作，方便测试退出信号是否真的传到了这一层。
func (f *fakeShutdowner) Shutdown(ctx context.Context) error {
	select {
	case f.shutdownCalled <- struct{}{}:
	default:
	}

	return nil
}
