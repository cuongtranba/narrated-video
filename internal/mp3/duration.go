// Package mp3 measures MPEG audio by walking its frame headers.
//
// Scene lengths in the rendered video are derived from audio duration, so that
// number is the timing contract for the whole render. Providers report their own
// durations and are sometimes wrong; this package is the only measurement the
// renderer trusts.
package mp3

import (
	"encoding/binary"
	"errors"
	"time"
)

// ErrNoFrames means nothing in the input parsed as MPEG audio.
var ErrNoFrames = errors.New("mp3: no valid MPEG audio frame found")

// Duration measures an MP3 by walking its frame headers. Providers report their
// own durations and are sometimes wrong; this is the only measurement trusted.
func Duration(data []byte) (time.Duration, error) {
	body := trimID3v1(data[skipID3v2(data):])

	pos, first, ok := findFrame(body, 0)
	if !ok {
		return 0, ErrNoFrames
	}

	// A VBR stream's own frame count is exact and cheap; walking it would still
	// be correct but would read the whole file.
	if frames, ok := vbrFrameCount(body[pos:], first); ok {
		return samplesDuration(int64(frames)*int64(first.samplesPerFrame), first.sampleRate), nil
	}

	var total time.Duration
	var samples int64
	rate := first.sampleRate

	for i := pos; i+4 <= len(body); {
		h, ok := parseFrameHeader(body[i:])
		if !ok {
			next, nh, found := findFrame(body, i+1)
			if !found {
				break
			}
			i, h = next, nh
		}
		// Sample rate may not change inside a well-formed stream, but a concatenated
		// file is still measurable if each run is accounted for at its own rate.
		if h.sampleRate != rate {
			total += samplesDuration(samples, rate)
			samples, rate = 0, h.sampleRate
		}
		samples += int64(h.samplesPerFrame)
		i += h.frameSize
	}

	return total + samplesDuration(samples, rate), nil
}

const (
	versionMPEG25    = 0
	versionReserved  = 1
	versionMPEG2     = 2
	versionMPEG1     = 3
	channelModeMono  = 3
	frameHeaderBytes = 4
)

type frameHeader struct {
	version         int
	layer           int
	bitrate         int
	sampleRate      int
	samplesPerFrame int
	frameSize       int
	channelMode     int
	crcProtected    bool
}

// Bitrates in kbps. Index 0 ("free") and 15 ("bad") are absent by construction:
// both are rejected before lookup.
var (
	bitratesV1L1  = [15]int{0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448}
	bitratesV1L2  = [15]int{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384}
	bitratesV1L3  = [15]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
	bitratesV2L1  = [15]int{0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256}
	bitratesV2L23 = [15]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}
)

var sampleRates = map[int][3]int{
	versionMPEG1:  {44100, 48000, 32000},
	versionMPEG2:  {22050, 24000, 16000},
	versionMPEG25: {11025, 12000, 8000},
}

func bitrateKbps(version, layer, index int) int {
	if version == versionMPEG1 {
		switch layer {
		case 1:
			return bitratesV1L1[index]
		case 2:
			return bitratesV1L2[index]
		default:
			return bitratesV1L3[index]
		}
	}
	if layer == 1 {
		return bitratesV2L1[index]
	}
	return bitratesV2L23[index]
}

func samplesPerFrame(version, layer int) int {
	switch layer {
	case 1:
		return 384
	case 2:
		return 1152
	default:
		if version == versionMPEG1 {
			return 1152
		}
		return 576
	}
}

func parseFrameHeader(b []byte) (frameHeader, bool) {
	if len(b) < frameHeaderBytes {
		return frameHeader{}, false
	}
	if b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return frameHeader{}, false
	}

	version := int(b[1]>>3) & 0x03
	if version == versionReserved {
		return frameHeader{}, false
	}
	layerBits := int(b[1]>>1) & 0x03
	if layerBits == 0 {
		return frameHeader{}, false
	}
	layer := 4 - layerBits

	bitrateIndex := int(b[2] >> 4)
	if bitrateIndex == 0 || bitrateIndex == 15 {
		return frameHeader{}, false
	}
	sampleRateIndex := int(b[2]>>2) & 0x03
	if sampleRateIndex == 3 {
		return frameHeader{}, false
	}

	h := frameHeader{
		version:         version,
		layer:           layer,
		bitrate:         bitrateKbps(version, layer, bitrateIndex) * 1000,
		sampleRate:      sampleRates[version][sampleRateIndex],
		samplesPerFrame: samplesPerFrame(version, layer),
		channelMode:     int(b[3] >> 6),
		crcProtected:    b[1]&0x01 == 0,
	}
	padding := int(b[2]>>1) & 0x01

	if h.layer == 1 {
		h.frameSize = (12*h.bitrate/h.sampleRate + padding) * 4
	} else {
		h.frameSize = (h.samplesPerFrame/8)*h.bitrate/h.sampleRate + padding
	}
	if h.frameSize <= frameHeaderBytes {
		return frameHeader{}, false
	}
	return h, true
}

// findFrame returns the first frame at or after start. A lone sync word occurs
// constantly inside audio payload and inside tags, so a candidate is accepted
// only when a matching header sits exactly one frame later — the first sync in
// a file is otherwise routinely the wrong one.
func findFrame(data []byte, start int) (int, frameHeader, bool) {
	if start < 0 {
		start = 0
	}
	for i := start; i+frameHeaderBytes <= len(data); i++ {
		if data[i] != 0xFF || data[i+1]&0xE0 != 0xE0 {
			continue
		}
		h, ok := parseFrameHeader(data[i:])
		if !ok {
			continue
		}
		next := i + h.frameSize
		if next+frameHeaderBytes > len(data) {
			return i, h, true
		}
		nh, ok := parseFrameHeader(data[next:])
		if ok && nh.version == h.version && nh.layer == h.layer && nh.sampleRate == h.sampleRate {
			return i, h, true
		}
	}
	return 0, frameHeader{}, false
}

// sideInfoSize is where a Xing/Info tag starts, measured from the end of the
// 4-byte header.
func sideInfoSize(h frameHeader) int {
	if h.version == versionMPEG1 {
		if h.channelMode == channelModeMono {
			return 17
		}
		return 32
	}
	if h.channelMode == channelModeMono {
		return 9
	}
	return 17
}

// vbrFrameCount reads the frame count a Xing/Info or VBRI header declares. Only
// Layer III carries these.
func vbrFrameCount(frame []byte, h frameHeader) (int, bool) {
	if h.layer != 3 {
		return 0, false
	}

	off := frameHeaderBytes + sideInfoSize(h)
	if h.crcProtected {
		off += 2
	}
	if off+8 <= len(frame) {
		switch string(frame[off : off+4]) {
		case "Xing", "Info":
			flags := binary.BigEndian.Uint32(frame[off+4 : off+8])
			if flags&0x1 != 0 && off+12 <= len(frame) {
				return int(binary.BigEndian.Uint32(frame[off+8 : off+12])), true
			}
			return 0, false
		}
	}

	// VBRI sits at a fixed offset from the frame start, independent of side info.
	const vbriOffset = 36
	if vbriOffset+18 <= len(frame) && string(frame[vbriOffset:vbriOffset+4]) == "VBRI" {
		return int(binary.BigEndian.Uint32(frame[vbriOffset+14 : vbriOffset+18])), true
	}
	return 0, false
}

func skipID3v2(data []byte) int {
	if len(data) < 10 || string(data[0:3]) != "ID3" {
		return 0
	}
	// A syncsafe integer never sets the high bit of any byte; if one is set this
	// is not a tag header we understand and the bytes are better left to resync.
	for _, b := range data[6:10] {
		if b&0x80 != 0 {
			return 0
		}
	}
	size := int(data[6])<<21 | int(data[7])<<14 | int(data[8])<<7 | int(data[9])
	end := 10 + size
	if data[5]&0x10 != 0 {
		end += 10 // footer
	}
	if end > len(data) {
		return len(data)
	}
	return end
}

func trimID3v1(data []byte) []byte {
	const tagSize = 128
	if len(data) >= tagSize && string(data[len(data)-tagSize:len(data)-tagSize+3]) == "TAG" {
		return data[:len(data)-tagSize]
	}
	return data
}

// samplesDuration converts a sample count without overflowing on long files.
func samplesDuration(samples int64, rate int) time.Duration {
	if samples <= 0 || rate <= 0 {
		return 0
	}
	r := int64(rate)
	return time.Duration(samples/r)*time.Second + time.Duration((samples%r)*int64(time.Second)/r)
}
