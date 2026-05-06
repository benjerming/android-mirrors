package scrcpy

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

// SessionAdb 抽象 Session 用到的 adb 操作，便于测试注入 fake。
type SessionAdb interface {
	Push(ctx context.Context, serial, local, remote string) error
	Reverse(ctx context.Context, serial, abstractName string, tcpPort int) error
	RemoveReverse(ctx context.Context, serial, abstractName string) error
	StartServer(ctx context.Context, serial string, opts ServerStartOpts) (*exec.Cmd, error)
}

// SessionOpts 是 OpenSession 的参数。
type SessionOpts struct {
	Serial   string
	Adb      SessionAdb
	JarBytes []byte
	Version  string
	Bitrate  int
	MaxFps   int
	MaxSize  int
}

const (
	abstractSocketName = "scrcpy"
	remoteJarPath      = "/data/local/tmp/scrcpy-server.jar"

	// probeEnvVar 是 readMeta 的诊断开关：值为空/0 关闭；
	// 值为 1 走默认 128 字节；值为 N（>0）按 N 字节抓取并落日志。
	probeEnvVar       = "SCRCPY_PROBE"
	probeDefaultBytes = 128

	// subscriberBuffer 是单个订阅者的 fragment 缓冲区大小。
	// 足够吞掉短暂的网络抖动；满了之后该订阅者会被丢帧（不影响其他订阅者）。
	subscriberBuffer = 256
)

// probeBytes 解析 SCRCPY_PROBE 环境变量并返回需要抓取的字节数；0 表示关闭。
func probeBytes() int {
	v := os.Getenv(probeEnvVar)
	if v == "" || v == "0" {
		return 0
	}
	if v == "1" {
		return probeDefaultBytes
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return probeDefaultBytes
	}
	return n
}

// ErrSessionClosed 表示会话已经关闭（或后台 pump 已终止）。
var ErrSessionClosed = errors.New("scrcpy: session closed")

// VideoSubscription 是一次订阅返回的句柄。
// Frames 中的每个元素都是一段完整的 fMP4 字节，顺序：
//
//	缓存的 init segment（如果已有） → 缓存的最近一帧 IDR fragment（如果已有） → 后续直播 fragment
//
// 调用方按到达顺序写出去即可。Cancel 退订；幂等。当会话关闭或 pump 退出时，Frames 会被关闭。
type VideoSubscription struct {
	Frames <-chan []byte
	Cancel func()
}

// Session 表示一次 scrcpy 会话：双 socket（video + control）+ 一个 server 进程。
//
// 视频流以 pub/sub 形式提供：会话内有一个常驻 pump goroutine 持有 video socket
// 持续读帧、muxer 转码 fMP4，再 fan-out 给所有订阅者。订阅者来去不影响 socket
// 也不影响其他订阅者。Close 必须由调用方显式调用。
type Session struct {
	opts     SessionOpts
	listener net.Listener
	cmd      *exec.Cmd
	video    net.Conn
	control  net.Conn
	jarPath  string

	width  uint16
	height uint16

	// state machine: Starting → Running → Dead
	state   atomic.Int32
	done    chan struct{} // closed by pump defer (sole owner)
	closeMu sync.Mutex    // serialises idempotent Close calls

	// pump / 订阅
	//
	// 同一个 pump 同时驱动两条 fan-out 通道：
	//   - subs / cachedInit / cachedKey      → fMP4（用于浏览器 MSE，老路径）
	//   - rawSubs / cachedRawKey             → 13B header + Annex-B（用于浏览器 WebCodecs，新路径）
	// 两条共用一把 subsMu，pump 退出时 defer 一次性关闭两边。
	subsMu       sync.Mutex
	subs         map[*subscriber]struct{} // fMP4 订阅者
	cachedInit   []byte                   // fMP4 init segment，pump 解析出后缓存
	cachedKey    []byte                   // 最近一帧 IDR fMP4 fragment
	rawSubs      map[*subscriber]struct{} // raw 订阅者
	cachedRawKey []byte                   // 最近一帧 IDR raw 帧（含 SPS+PPS+IDR），新订阅者立即 prime VideoDecoder
}

type subscriber struct {
	ch chan []byte
}

// State returns the current session lifecycle state.
func (s *Session) State() SessionState {
	return SessionState(s.state.Load())
}

// Done returns a channel that is closed when the session transitions to Dead
// (pump has exited and all subscriber channels have been closed).
func (s *Session) Done() <-chan struct{} { return s.done }

// transitionTo atomically attempts to move state to next; returns true on success.
// Illegal transitions (per SessionState.CanTransition) return false without changing state.
func (s *Session) transitionTo(next SessionState) bool {
	for {
		cur := SessionState(s.state.Load())
		if !cur.CanTransition(next) {
			return false
		}
		if s.state.CompareAndSwap(int32(cur), int32(next)) {
			return true
		}
	}
}

// OpenSession 启动 scrcpy 会话：
//  1. 把 jar 写到临时文件 → adb push 到设备
//  2. 服务端 listen ephemeral 端口 → adb reverse
//  3. 启动 scrcpy-server 进程
//  4. accept 两次连接（video + control）
//  5. 读 device meta + codec meta
//  6. 起常驻 pump goroutine
//
// 任一步失败都会回滚已申请的资源后返回错误。
func OpenSession(ctx context.Context, opts SessionOpts) (*Session, error) {
	if opts.Adb == nil {
		return nil, errors.New("scrcpy: SessionOpts.Adb is required")
	}
	if opts.Bitrate == 0 {
		opts.Bitrate = 4_000_000
	}
	if opts.MaxFps == 0 {
		opts.MaxFps = 30
	}

	jarFile, err := os.CreateTemp("", "scrcpy-server-*.jar")
	if err != nil {
		return nil, fmt.Errorf("temp jar: %w", err)
	}
	if _, err := jarFile.Write(opts.JarBytes); err != nil {
		jarFile.Close()
		os.Remove(jarFile.Name())
		return nil, err
	}
	jarFile.Close()
	jarPath := jarFile.Name()

	if err := opts.Adb.Push(ctx, opts.Serial, jarPath, remoteJarPath); err != nil {
		os.Remove(jarPath)
		return nil, fmt.Errorf("adb push: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Remove(jarPath)
		return nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port

	if err := opts.Adb.Reverse(ctx, opts.Serial, abstractSocketName, port); err != nil {
		ln.Close()
		os.Remove(jarPath)
		return nil, fmt.Errorf("adb reverse: %w", err)
	}

	cmd, err := opts.Adb.StartServer(ctx, opts.Serial, ServerStartOpts{
		Version:    opts.Version,
		BitrateBps: opts.Bitrate,
		MaxFps:     opts.MaxFps,
		MaxSize:    opts.MaxSize,
	})
	if err != nil {
		_ = opts.Adb.RemoveReverse(ctx, opts.Serial, abstractSocketName)
		ln.Close()
		os.Remove(jarPath)
		return nil, fmt.Errorf("start server: %w", err)
	}

	s := &Session{
		opts:     opts,
		listener: ln,
		cmd:      cmd,
		jarPath:  jarPath,
		subs:     make(map[*subscriber]struct{}),
		rawSubs:  make(map[*subscriber]struct{}),
		done:     make(chan struct{}),
	}
	s.state.Store(int32(StateStarting))

	if err := s.acceptConns(); err != nil {
		// Early failure: inline cleanup (pump never started, done never signalled).
		if s.video != nil {
			s.video.Close()
		}
		if s.control != nil {
			s.control.Close()
		}
		ln.Close()
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		_ = opts.Adb.RemoveReverse(ctx, opts.Serial, abstractSocketName)
		os.Remove(jarPath)
		return nil, err
	}
	if err := s.readMeta(); err != nil {
		// Early failure: inline cleanup (pump never started, done never signalled).
		if s.video != nil {
			s.video.Close()
		}
		if s.control != nil {
			s.control.Close()
		}
		ln.Close()
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		_ = opts.Adb.RemoveReverse(ctx, opts.Serial, abstractSocketName)
		os.Remove(jarPath)
		return nil, err
	}

	s.transitionTo(StateRunning)
	go s.pump()
	return s, nil
}

func (s *Session) acceptConns() error {
	v, err := s.listener.Accept()
	if err != nil {
		return fmt.Errorf("accept video: %w", err)
	}
	s.video = v
	c, err := s.listener.Accept()
	if err != nil {
		return fmt.Errorf("accept control: %w", err)
	}
	s.control = c
	return nil
}

func (s *Session) readMeta() error {
	// scrcpy 3.0 video socket（audio=false control=true）协议布局，
	// 由 hex dump 在本机实测确认：
	//   00..3F (64B) 设备名（NUL 填充；没有 dummy byte）
	//   40..43 (4B)  codec_id  e.g. 'h264' = 0x68323634
	//   44..47 (4B)  width  (BE u32)
	//   48..4B (4B)  height (BE u32)
	//   4C+         帧循环：8B PTS + 4B size + size 字节 NAL（Annex-B 含 start code）
	//
	// 调试钩子：设置 SCRCPY_PROBE=1（或 =N 指定字节数）时，把 video 头若干字节
	// 用 TeeReader 透传到日志再继续解析；解析逻辑不受影响。下次协议再变可直接复用。
	src := io.Reader(s.video)
	if probeN := probeBytes(); probeN > 0 {
		var buf bytes.Buffer
		src = io.TeeReader(s.video, &buf)
		defer func() {
			b := buf.Bytes()
			if len(b) > probeN {
				b = b[:probeN]
			}
			slog.Warn("scrcpy.session: video meta probe",
				"bytes", len(b),
				"hex", hex.EncodeToString(b),
			)
		}()
	}
	name := make([]byte, 64)
	if _, err := io.ReadFull(src, name); err != nil {
		return fmt.Errorf("read device name: %w", err)
	}
	cm := make([]byte, 12)
	if _, err := io.ReadFull(src, cm); err != nil {
		return fmt.Errorf("read codec meta: %w", err)
	}
	w := binary.BigEndian.Uint32(cm[4:8])
	h := binary.BigEndian.Uint32(cm[8:12])
	if w == 0 || h == 0 || w > 0xFFFF || h > 0xFFFF {
		return fmt.Errorf("invalid screen dimensions %dx%d", w, h)
	}
	s.width = uint16(w)
	s.height = uint16(h)
	return nil
}

// Subscribe 注册一个新订阅者。
//
// 实现细节：
//   - 已经缓存的 init segment 与最近一帧 IDR fragment 会先入队，让晚到的订阅者立刻能 prime MSE，
//     不必等下一个 IDR。
//   - Frames channel 是 bounded buffer；写不进去（该订阅者太慢）就直接丢，只影响这个订阅者，
//     浏览器在下一次 IDR 时自动 resync。
//   - 调用方负责消费 Frames 直到关闭，并最终调用 Cancel（或 ctx 取消时通过 defer 调）。
func (s *Session) Subscribe(ctx context.Context) (*VideoSubscription, error) {
	if st := s.State(); st == StateClosing || st == StateDead {
		return nil, ErrSessionClosed
	}

	sub := &subscriber{ch: make(chan []byte, subscriberBuffer)}

	s.subsMu.Lock()
	if s.subs == nil {
		// pump 已退出；没法继续广播
		s.subsMu.Unlock()
		return nil, ErrSessionClosed
	}
	if s.cachedInit != nil {
		// buffer 是空的且容量 ≥ 2，下面这两次写不会阻塞
		sub.ch <- s.cachedInit
	}
	if s.cachedKey != nil {
		sub.ch <- s.cachedKey
	}
	s.subs[sub] = struct{}{}
	SubscribersGauge.WithLabelValues(s.opts.Serial).Inc()
	s.subsMu.Unlock()

	cancel := func() {
		s.subsMu.Lock()
		if s.subs != nil {
			if _, ok := s.subs[sub]; ok {
				delete(s.subs, sub)
				close(sub.ch)
				SubscribersGauge.WithLabelValues(s.opts.Serial).Dec()
			}
		}
		s.subsMu.Unlock()
	}

	// 监听 ctx，自动清理。这样调用方即使忘了 defer Cancel 也不会泄漏。
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-s.done:
		}
	}()

	return &VideoSubscription{Frames: sub.ch, Cancel: cancel}, nil
}

// SubscribeRaw 注册 raw（13B header + Annex-B）订阅者。
// bootstrap 时把 cachedRawKey（最近一帧自包含 IDR：SPS+PPS+IDR）入队，
// 让新订阅者立即能 configure VideoDecoder。如果该 IDR 比当前直播 P 帧早太多，
// 头几秒可能花屏，下个自然 IDR 自愈。
//
// 之前这里曾向 scrcpy server 发 RESET_VIDEO 控制消息以拿一个新鲜 IDR，
// 但常量 byte 值未经实测验证，被这版 scrcpy server 解读成另一种 type 并继续从
// control socket 读 payload，吞掉后续 tap 字节，导致点击失灵 + 设备弹
// "shell pasted from your clipboard"。已撤回；改 IDR 周期由 scrcpy 启动参数控制。
func (s *Session) SubscribeRaw(ctx context.Context) (*VideoSubscription, error) {
	if st := s.State(); st == StateClosing || st == StateDead {
		return nil, ErrSessionClosed
	}

	sub := &subscriber{ch: make(chan []byte, subscriberBuffer)}

	s.subsMu.Lock()
	if s.rawSubs == nil {
		s.subsMu.Unlock()
		return nil, ErrSessionClosed
	}
	if s.cachedRawKey != nil {
		sub.ch <- s.cachedRawKey
	}
	s.rawSubs[sub] = struct{}{}
	SubscribersGauge.WithLabelValues(s.opts.Serial).Inc()
	s.subsMu.Unlock()

	cancel := func() {
		s.subsMu.Lock()
		if s.rawSubs != nil {
			if _, ok := s.rawSubs[sub]; ok {
				delete(s.rawSubs, sub)
				close(sub.ch)
				SubscribersGauge.WithLabelValues(s.opts.Serial).Dec()
			}
		}
		s.subsMu.Unlock()
	}

	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-s.done:
		}
	}()

	return &VideoSubscription{Frames: sub.ch, Cancel: cancel}, nil
}

// pump 是会话内唯一持有 video socket 读权的 goroutine。
// 每读到一帧 Annex-B → 同时 fan-out 两路：fMP4（老 MSE 路径）+ raw（新 WebCodecs 路径）。
// 遇到读错误或 socket 关闭即退出。
func (s *Session) pump() {
	defer func() {
		s.subsMu.Lock()
		for sub := range s.subs {
			close(sub.ch)
			SubscribersGauge.WithLabelValues(s.opts.Serial).Dec()
		}
		for sub := range s.rawSubs {
			close(sub.ch)
			SubscribersGauge.WithLabelValues(s.opts.Serial).Dec()
		}
		s.subs = nil
		s.rawSubs = nil
		s.subsMu.Unlock()
		s.transitionTo(StateDead)
		close(s.done)
	}()

	mux := NewFmp4Muxer()
	// raw 路径的 CONFIG/IDR merge 策略，对齐 scrcpy-mask / Gamma Tauri 实现：
	//   - CONFIG 帧（PTS bit63 = 1，仅含 SPS/PPS）→ 仅缓存到 configBuf，**不广播**
	//   - 下一帧 IDR → 把 configBuf 整段 prepend 到 IDR 前面再广播，然后清空 configBuf
	// 这样订阅者收到的每个 IDR 都是自包含的 SPS+PPS+IDR，且 SPS/PPS 一定来自配套的
	// CONFIG，不会出现"老 SPS 配新 IDR"的竞态——这正是花屏的根因。
	// fMP4 路径不受影响（init segment 由 muxer 单独管理）。
	var configBuf []byte
	// pts_base：把第一个非 CONFIG 帧的 PTS 作为基准，后续帧广播相对时间戳。
	// 避免某些编码器（MTK OMX）的绝对 PTS 接近 2^61 边界、超出 JS Number.MAX_SAFE_INTEGER。
	var ptsBase uint64
	ptsBaseSet := false
	for {
		hdr := make([]byte, 12)
		if _, err := io.ReadFull(s.video, hdr); err != nil {
			if s.State() == StateRunning {
				slog.Warn("scrcpy.session: pump read header failed",
					"serial", s.opts.Serial, "error", err)
			}
			return
		}
		// scrcpy 协议在 PTS 高 2 bit 塞 flag：bit63=CONFIG，bit62=KEY_FRAME。
		// 实际 PTS 用低 62 bit。不 mask 的话 timestamp 会超出 JS Number.MAX_SAFE_INTEGER
		// (2^53)，WebCodecs decoder 拿到非法时间戳后静默丢帧、无 output、无 error。
		const ptsMask uint64 = (1 << 62) - 1
		rawPts := binary.BigEndian.Uint64(hdr[0:8])
		isConfigFlag := (rawPts>>63)&1 != 0
		isKeyFlag := (rawPts>>62)&1 != 0
		ptsAbs := rawPts & ptsMask
		size := binary.BigEndian.Uint32(hdr[8:12])
		frame := make([]byte, size)
		if _, err := io.ReadFull(s.video, frame); err != nil {
			if s.State() == StateRunning {
				slog.Warn("scrcpy.session: pump read frame failed",
					"serial", s.opts.Serial, "error", err)
			}
			return
		}
		init, frag, err := mux.WriteFrame(frame, ptsAbs)
		if err != nil {
			slog.Warn("scrcpy.session: muxer error",
				"serial", s.opts.Serial, "error", err)
			return
		}
		nals := splitAnnexB(frame)
		// 关键帧判断双保险：先看 scrcpy PTS bit62 的 KEY_FRAME flag，再扫 NAL type 5 兜底。
		// 仅看 type 5 会漏判 MediaCodec 偶发的"软关键帧"（intra-only I 帧但 NAL 是 type 1），
		// 这种帧被误判成 delta 后，前端解码器以为参考链未断、用错参考帧，导致花屏。
		isKey := isKeyFlag || isKeyFrame(nals)

		// fMP4 路径维持原行为（init+frag 都广播）。
		if len(init) > 0 {
			s.broadcastInit(init)
		}
		if len(frag) > 0 {
			s.broadcastFrag(frag, isKey)
		}

		// raw 路径：CONFIG 帧只缓存不广播；IDR 时把 cache prepend 上去再发。
		// 这样订阅者收到的每个 IDR 都是自包含的 SPS+PPS+IDR，且 SPS/PPS 一定来自配套的
		// CONFIG —— 避免老 SPS 配新 IDR 导致部分宏块花屏。
		if isConfigFlag {
			configBuf = append(configBuf[:0], frame...)
			continue
		}
		rawAnnexB := frame
		if isKey && len(configBuf) > 0 {
			rawAnnexB = prependBytes(configBuf, frame)
			configBuf = configBuf[:0] // 用完即清，下次 RESET_VIDEO 后会有新 CONFIG
		}
		// PTS 归一化：第一个非 CONFIG 帧的 ptsAbs 作为基准，后续广播相对时间戳。
		if !ptsBaseSet {
			ptsBase = ptsAbs
			ptsBaseSet = true
		}
		var ptsRel uint64
		if ptsAbs > ptsBase {
			ptsRel = ptsAbs - ptsBase
		}
		s.broadcastRaw(buildRawFrame(ptsRel, rawAnnexB, isKey), isKey)
	}
}

// prependBytes 把 prefix 拼到 frame 前面，返回新的 buffer。两份输入都以原样字节复制。
func prependBytes(prefix, frame []byte) []byte {
	out := make([]byte, len(prefix)+len(frame))
	copy(out, prefix)
	copy(out[len(prefix):], frame)
	return out
}

// buildRawFrame 拼前端约定的 13B header + Annex-B 帧。见 web/src/lib/mirror/transport.ts。
//
//	offset 0   1B  flags (bit0=isKey)
//	offset 1   8B  pts_micros (BE u64)
//	offset 9   4B  payload_len (BE u32)
//	offset 13  N   annex_b_data
func buildRawFrame(ptsMicros uint64, annexB []byte, isKey bool) []byte {
	out := make([]byte, 13+len(annexB))
	if isKey {
		out[0] = 0x01
	}
	binary.BigEndian.PutUint64(out[1:9], ptsMicros)
	binary.BigEndian.PutUint32(out[9:13], uint32(len(annexB)))
	copy(out[13:], annexB)
	return out
}

func (s *Session) broadcastInit(b []byte) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	s.cachedInit = b
	for sub := range s.subs {
		s.deliverLocked(sub, b)
	}
}

func (s *Session) broadcastFrag(b []byte, isKey bool) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	if isKey {
		s.cachedKey = b
	}
	for sub := range s.subs {
		s.deliverLocked(sub, b)
	}
}

// broadcastRaw fan-out 13B header + Annex-B 给 rawSubs；isKey=true 时缓存为 bootstrap 帧。
func (s *Session) broadcastRaw(b []byte, isKey bool) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	if isKey {
		s.cachedRawKey = b
	}
	for sub := range s.rawSubs {
		s.deliverLocked(sub, b)
	}
}

// deliverLocked 非阻塞写入；满了直接丢。subsMu 必须已经持有。
func (s *Session) deliverLocked(sub *subscriber, b []byte) {
	select {
	case sub.ch <- b:
	default:
		// 慢消费者；当前帧丢弃。下一次 IDR 浏览器自然会 resync。
		FramesDroppedTotal.WithLabelValues(s.opts.Serial).Inc()
		slog.Warn("scrcpy.session: subscriber buffer full, dropping frame",
			"serial", s.opts.Serial)
	}
}

// SendControl 把控制字节写入 control socket。
func (s *Session) SendControl(payload []byte) error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if st := s.State(); st == StateClosing || st == StateDead || s.control == nil {
		return ErrSessionClosed
	}
	_, err := s.control.Write(payload)
	return err
}

// Width / Height 返回设备屏幕分辨率（codec meta）。
func (s *Session) Width() uint16  { return s.width }
func (s *Session) Height() uint16 { return s.height }

// Close 释放会话所有资源：关 socket（pump 由此感知到 EOF 退出）、kill 进程、撤销 reverse、删 jar。
// 幂等，并发安全。返回前会等 pump 退出，保证所有订阅者的 channel 都已关闭。
func (s *Session) Close() {
	s.closeMu.Lock()
	// Already in teardown or done — just wait for pump to finish.
	if st := s.State(); st == StateClosing || st == StateDead {
		s.closeMu.Unlock()
		<-s.done
		return
	}
	if !s.transitionTo(StateClosing) {
		// Lost a race with pump's natural exit (Running→Dead). Just wait.
		s.closeMu.Unlock()
		<-s.done
		return
	}
	// Now we own the teardown. Close sockets — pump will read EOF and its defer
	// will transition Closing→Dead and close(s.done).
	if s.video != nil {
		s.video.Close()
	}
	if s.control != nil {
		s.control.Close()
	}
	if s.listener != nil {
		s.listener.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
	if s.opts.Adb != nil {
		_ = s.opts.Adb.RemoveReverse(context.Background(), s.opts.Serial, abstractSocketName)
	}
	if s.jarPath != "" {
		_ = os.Remove(s.jarPath)
	}
	s.closeMu.Unlock()
	<-s.done
}
