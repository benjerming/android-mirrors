package scrcpy

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/mp4"
)

// fmp4Timescale 是 video track 的标准 timescale。
// 90000 是 RTP/MPEG-TS 行业惯例，对 30/60fps 都能整除。
const fmp4Timescale = 90000

// ErrMissingSpsPps 表示首帧没有同时包含 SPS+PPS，无法构造 init segment。
var ErrMissingSpsPps = errors.New("fmp4: first frame missing SPS/PPS")

// Fmp4Muxer 把一帧 H.264 Annex-B 字节序列转成 fMP4：
// 首帧返回 init segment（ftyp+moov）+ 第一个 fragment（moof+mdat）；
// 后续帧只返回 fragment。无重编码，纯字节拼装。
//
// 不是线程安全的：调用方保证一个 muxer 一个 goroutine 写。
type Fmp4Muxer struct {
	initWritten bool
	seqNum      uint32
	sps         []byte // 不含 start code
	pps         []byte // 不含 start code
}

func NewFmp4Muxer() *Fmp4Muxer { return &Fmp4Muxer{} }

// WriteFrame 接收一帧（一个或多个 Annex-B NAL，带 start code），返回：
// - init segment 字节（仅首帧非空）
// - fragment 字节
// - error
//
// ptsMicros 是该帧 PTS 的微秒值（来自 scrcpy header）。
func (m *Fmp4Muxer) WriteFrame(frame []byte, ptsMicros uint64) ([]byte, []byte, error) {
	nals := splitAnnexB(frame)
	if !m.initWritten {
		for _, n := range nals {
			switch nalType(n) {
			case 7: // SPS
				m.sps = append([]byte(nil), n...)
			case 8: // PPS
				m.pps = append([]byte(nil), n...)
			}
		}
		if m.sps == nil || m.pps == nil {
			return nil, nil, ErrMissingSpsPps
		}
		// avc.ParseSPSNALUnit 期望不带 start code 的纯 NAL；splitAnnexB 已经剥掉了。
		if _, err := avc.ParseSPSNALUnit(m.sps, true); err != nil {
			return nil, nil, fmt.Errorf("parse SPS: %w", err)
		}
		init := mp4.CreateEmptyInit()
		trak := init.AddEmptyTrack(fmp4Timescale, "video", "und")
		if err := trak.SetAVCDescriptor("avc1", [][]byte{m.sps}, [][]byte{m.pps}, true); err != nil {
			return nil, nil, fmt.Errorf("SetAVCDescriptor: %w", err)
		}
		var initBuf bytes.Buffer
		if err := init.Encode(&initBuf); err != nil {
			return nil, nil, fmt.Errorf("encode init: %w", err)
		}
		fragBuf, err := m.buildFragment(nals, ptsMicros, true)
		if err != nil {
			return nil, nil, err
		}
		m.initWritten = true
		return initBuf.Bytes(), fragBuf, nil
	}
	fragBuf, err := m.buildFragment(nals, ptsMicros, isKeyFrame(nals))
	if err != nil {
		return nil, nil, err
	}
	return nil, fragBuf, nil
}

func (m *Fmp4Muxer) buildFragment(nals [][]byte, ptsMicros uint64, isKey bool) ([]byte, error) {
	m.seqNum++
	frag, err := mp4.CreateFragment(m.seqNum, 1)
	if err != nil {
		return nil, err
	}
	// 把每个 NAL 转成 length-prefixed (4B big-endian length + payload)，组成一个 sample。
	// SPS/PPS（已在 init 里）和 AUD（type 9）都跳过。
	var sample bytes.Buffer
	for _, n := range nals {
		t := nalType(n)
		if t == 7 || t == 8 || t == 9 {
			continue
		}
		var lenBuf [4]byte
		lenBuf[0] = byte(len(n) >> 24)
		lenBuf[1] = byte(len(n) >> 16)
		lenBuf[2] = byte(len(n) >> 8)
		lenBuf[3] = byte(len(n))
		sample.Write(lenBuf[:])
		sample.Write(n)
	}
	// 假设 30fps；准确 dur 对 MSE 顺序播放不关键。timescale=90000，30fps → 3000。
	const dur uint32 = fmp4Timescale / 30
	// flags 的高位 bit 5 (0x02000000) 是 sample_is_non_sync_sample=0，即关键帧。
	// 非关键帧用 0x01010000（is_non_sync=1, depends_on=2）。
	var flags uint32 = 0x01010000
	if isKey {
		flags = 0x02000000
	}
	fs := mp4.FullSample{
		Sample: mp4.Sample{
			Dur:                   dur,
			Size:                  uint32(sample.Len()),
			Flags:                 flags,
			CompositionTimeOffset: 0,
		},
		DecodeTime: ptsMicros * fmp4Timescale / 1_000_000,
		Data:       sample.Bytes(),
	}
	if err := frag.AddFullSampleToTrack(fs, 1); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := frag.Encode(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// splitAnnexB 切分 Annex-B 字节流为 NAL 单元（每段已剥掉 start code）。
//
// 支持 3 字节和 4 字节 start code（00 00 01 / 00 00 00 01）。
func splitAnnexB(b []byte) [][]byte {
	var out [][]byte
	i := 0
	for i < len(b) {
		sc := findStartCode(b, i)
		if sc < 0 {
			break
		}
		start := sc + 4
		if sc+2 < len(b) && b[sc+2] == 1 {
			start = sc + 3
		}
		next := findStartCode(b, start)
		if next < 0 {
			out = append(out, b[start:])
			break
		}
		out = append(out, b[start:next])
		i = next
	}
	return out
}

func findStartCode(b []byte, from int) int {
	for i := from; i+2 < len(b); i++ {
		if b[i] != 0 || b[i+1] != 0 {
			continue
		}
		if b[i+2] == 1 {
			return i
		}
		if b[i+2] == 0 && i+3 < len(b) && b[i+3] == 1 {
			return i
		}
	}
	return -1
}

func nalType(nal []byte) byte {
	if len(nal) == 0 {
		return 0
	}
	return nal[0] & 0x1F
}

func isKeyFrame(nals [][]byte) bool {
	for _, n := range nals {
		if nalType(n) == 5 {
			return true
		}
	}
	return false
}
