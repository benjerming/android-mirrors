# TODO

##

## 前端刷新网页后，无法重连（后端bug）

## 模拟器是服务器子进程，会跟随服务器一起死亡，但服务器在数据库中额外记录了模拟器状态，必须在关闭服务器之前，手动关闭所有模拟器

## 镜像 WebCodecs 收尾（可选）

raw 通道（`?fmt=raw`，13B header + Annex-B）已经上线，是默认路径；fMP4 路径保留作为 `?fmt=fmp4` 兜底。
观察一段时间没人触发 fmp4 路径后，可以整文件删除 `internal/infrastructure/scrcpy/fmp4.go` + `fmp4_test.go`，
顺带把 `Session.subs/cachedInit/cachedKey/Subscribe/broadcastInit/broadcastFrag` 和 `SessionManager.AttachVideo` 一起清掉，
只留 raw 路径。pump 也能少做一次 muxer 调用。


