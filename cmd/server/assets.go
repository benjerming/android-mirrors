package main

import (
	"embed"
	"io/fs"
	"net/http"
)

// embeddedDist 是把 cmd/server/dist/ 整个目录打进二进制的入口。
//
// 几个关键点：
//   - 用 `all:` 前缀，是为了让 .gitkeep 这类点开头的文件也被嵌入；
//     Make 在 build 之前会把真正的 web/dist 拷到这里，dev 时这里只有 .gitkeep 占位。
//   - 直接放在 cmd/server 下，是因为 //go:embed 只能嵌入"和当前 Go 文件同一目录树"
//     里的文件，无法跨包引用上级目录里的 web/dist。
//   - 不再用 build tag 切换 dev/prod：始终嵌入 + 运行时检测"是否真有 index.html"，
//     这样同一个二进制能同时支持 dev（dist 为空、SPA 由 Vite 提供）和 prod。
//
//go:embed all:dist
var embeddedDist embed.FS

// assetsFS 用来把内嵌的 dist 包成 HTTP 可读的文件系统。
//
// 调用方一般不需要先判断 dist 是不是真的有内容——
// router 那一层会通过探测 index.html 是否存在来决定要不要挂 SPA 路由，
// 所以这里只负责"把 embed.FS 适配成 http.FileSystem"。
func assetsFS() http.FileSystem {
	sub, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		return nil
	}
	return http.FS(sub)
}
