package scrcpy

import _ "embed"

//go:embed assets/scrcpy-server.jar
var EmbeddedServerJar []byte

// ServerVersion 必须和 assets/scrcpy-server.jar 的实际版本一致。
// scrcpy 协议要求启动参数第一个传 server 版本号字符串，否则进程立即退出。
const ServerVersion = "3.0"
