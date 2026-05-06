// Command server 是后端二进制的入口包。
//
// 它本身不放业务逻辑，只负责：解析命令行参数、决定开发/生产配置、把整个应用装配出来，
// 然后监听信号让 App 自己做优雅停机。真正的业务装配看 internal/bootstrap。
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"assassin-android-controller/internal/bootstrap"
)

// version 表示构建版本号；release 构建会通过 -ldflags "-X main.version=..." 注入真实值。
// dev 模式下保持 "dev" 占位，便于后续 /version 等接口区分构建环境。
var version = "dev"

// main 是后端服务的入口点。
//
// 这里只做一件事：把真正的启动逻辑委托给 run()，根据返回值决定退出码。
// 选择"main 只决定退出码、run 才负责报错"这个模式，是为了让所有 defer 都能正常执行——
// 直接在 main 里调用 log.Fatalf / os.Exit 会跳过 defer，遗留资源（信号监听、数据库连接等）。
func main() {
	if err := run(); err != nil {
		// 写到 stderr 即可：到这里通常说明 logger 还没建好（早期错误），
		// 或者 logger 已经把详细信息打过一次了，这里只补一行供 systemd / 终端看。
		_, _ = os.Stderr.WriteString("server exited with error: " + err.Error() + "\n")
		os.Exit(1)
	}
}

// run 用来真正启动后端 HTTP 服务，所有 defer（信号停止、DB 关闭等）都在这一层完成。
//
// 启动流程拆成 3 步：
//  1. 先解析命令行参数（目前只有 -config）。
//  2. 用解析得到的配置文件路径装配整个应用（数据库、仓储、服务、路由）。
//  3. 监听 SIGINT / SIGTERM，在收到退出信号时让 App 自己做优雅停机。
//
// 装配前的错误（参数解析失败、配置文件读不到）只能用标准库 log 打印——这时 slog logger 还没构建出来。
// 装配完成之后的错误才会走 app.Logger，确保线上环境能拿到结构化日志。
func run() error {
	configPath, err := resolveConfigPath(os.Args[1:])
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}

	// 信号 ctx 既用于装配阶段，也会一路传给 Serve；这样在初始化阶段（比如灌种子模板）被 ctrl+c 也能立即返回。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewApp(ctx, configPath, assetsFS())
	if err != nil {
		return fmt.Errorf("bootstrap app: %w", err)
	}

	app.Logger.Info("starting server",
		"version", version,
		"config", configPath,
		"env", app.Config.App.Env,
	)

	if err := app.Run(ctx); err != nil {
		app.Logger.Error("run app", "error", err)
		return err
	}
	return nil
}

// defaultConfigPath 是没有命令行参数、也没有环境变量时使用的兜底配置文件路径。
//
// 选择 dev 配置作为兜底：
//   - 90% 以上的"无参数启动"发生在本地开发，比如 `go run ./cmd/server`。
//   - 部署侧（systemd / docker / k8s）一向会显式传 -config 或设置 APP_CONFIG，
//     不会依赖这个默认值，所以默认指向 dev 不会污染生产环境。
const defaultConfigPath = "configs/config.dev.yaml"

// configPathEnvVar 表示用来覆盖默认配置路径的环境变量名。
// 部署时推荐：APP_CONFIG=/etc/assassin/config.yaml ./assassin-android-controller
const configPathEnvVar = "APP_CONFIG"

// resolveConfigPath 用来解析启动参数，决定这次启动要读取哪份配置文件。
//
// 优先级（从低到高）：
//  1. 编译期常量 defaultConfigPath。
//  2. 环境变量 APP_CONFIG。
//  3. 命令行 -config 参数。
//
// flag 输出被重定向到 io.Discard，避免在测试或 systemd 日志里看到默认的帮助文案。
func resolveConfigPath(args []string) (string, error) {
	flagSet := flag.NewFlagSet("server", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	// 把"环境变量 fallback"折进 flag 默认值里：用户传 -config 时直接覆盖；
	// 没传 -config 但设了环境变量时，环境变量胜出；都没设才用编译期常量。
	fallback := defaultConfigPath
	if v := os.Getenv(configPathEnvVar); v != "" {
		fallback = v
	}

	configPath := flagSet.String("config", fallback, "config file path")
	if err := flagSet.Parse(args); err != nil {
		return "", err
	}

	return *configPath, nil
}
