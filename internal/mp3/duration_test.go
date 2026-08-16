package mp3

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

// frameSpec describes a frame the test synthesizes, in the same terms the header
// bits carry, so a case can be read against the MPEG tables directly.
type frameSpec struct {
	version         int // versionMPEG1 / versionMPEG2 / versionMPEG25
	layer           int // 1, 2, 3
	bitrateIndex    int
	sampleRateIndex int
	padding         int
	channelMode     int
}

func (s frameSpec) sampleRate() int { return sampleRates[s.version][s.sampleRateIndex] }

func (s frameSpec) samplesPerFrame() int { return samplesPerFrame(s.version, s.layer) }

func (s frameSpec) size() int {
	bitrate := bitrateKbps(s.version, s.layer, s.bitrateIndex) * 1000
	if s.layer == 1 {
		return (12*bitrate/s.sampleRate() + s.padding) * 4
	}
	return (s.samplesPerFrame()/8)*bitrate/s.sampleRate() + s.padding
}

// build emits a whole frame: a header assembled bit by bit, then a zeroed
// payload (which is also what a genuine silent Layer III frame looks like).
func (s frameSpec) build() []byte {
	frame := make([]byte, s.size())
	frame[0] = 0xFF
	frame[1] = 0xE0 |
		byte(s.version)<<3 |
		byte(4-s.layer)<<1 |
		0x01 // protection bit set == no CRC
	frame[2] = byte(s.bitrateIndex)<<4 |
		byte(s.sampleRateIndex)<<2 |
		byte(s.padding)<<1
	frame[3] = byte(s.channelMode) << 6
	return frame
}

func (s frameSpec) repeat(n int) []byte {
	var buf bytes.Buffer
	frame := s.build()
	for range n {
		buf.Write(frame)
	}
	return buf.Bytes()
}

// cbr441k128 is the shape the silence provider emits and the most common MP3 in
// the wild: MPEG-1 Layer III, 44.1 kHz, 128 kbps, mono, padded.
var cbr441k128 = frameSpec{
	version:         versionMPEG1,
	layer:           3,
	bitrateIndex:    9, // 128 kbps
	sampleRateIndex: 0, // 44100 Hz
	padding:         1,
	channelMode:     channelModeMono,
}

func id3v2(payload int) []byte {
	tag := make([]byte, 10+payload)
	copy(tag, "ID3")
	tag[3], tag[4] = 0x03, 0x00
	// Syncsafe size: 7 bits per byte.
	tag[6] = byte(payload >> 21 & 0x7F)
	tag[7] = byte(payload >> 14 & 0x7F)
	tag[8] = byte(payload >> 7 & 0x7F)
	tag[9] = byte(payload & 0x7F)
	return tag
}

func id3v1() []byte {
	tag := make([]byte, 128)
	copy(tag, "TAG")
	return tag
}

// xingFrame builds a Layer III frame carrying a Xing header that declares
// frameCount, which must win over however many frames actually follow.
func xingFrame(s frameSpec, frameCount int) []byte {
	frame := s.build()
	off := frameHeaderBytes + sideInfoSize(frameHeader{version: s.version, channelMode: s.channelMode})
	copy(frame[off:], "Xing")
	binary.BigEndian.PutUint32(frame[off+4:], 0x1) // frames field present
	binary.BigEndian.PutUint32(frame[off+8:], uint32(frameCount))
	return frame
}

func vbriFrame(s frameSpec, frameCount int) []byte {
	frame := s.build()
	const vbriOffset = 36
	copy(frame[vbriOffset:], "VBRI")
	binary.BigEndian.PutUint32(frame[vbriOffset+14:], uint32(frameCount))
	return frame
}

func framesDuration(s frameSpec, frames int) time.Duration {
	return samplesDuration(int64(frames)*int64(s.samplesPerFrame()), s.sampleRate())
}

func TestDuration(t *testing.T) {
	stereo441k := frameSpec{version: versionMPEG1, layer: 3, bitrateIndex: 9, sampleRateIndex: 0, channelMode: 0}

	mpeg2L3 := frameSpec{
		version:         versionMPEG2,
		layer:           3,
		bitrateIndex:    8, // 64 kbps
		sampleRateIndex: 0, // 22050 Hz
		channelMode:     channelModeMono,
	}
	layerI := frameSpec{version: versionMPEG1, layer: 1, bitrateIndex: 4, sampleRateIndex: 0, channelMode: 0}
	layerII := frameSpec{version: versionMPEG1, layer: 2, bitrateIndex: 8, sampleRateIndex: 0, channelMode: 0}

	tests := []struct {
		name string
		data []byte
		want time.Duration
	}{
		{
			name: "cbr 44.1kHz 128kbps",
			data: cbr441k128.repeat(383),
			want: framesDuration(cbr441k128, 383),
		},
		{
			name: "cbr single frame",
			data: cbr441k128.repeat(1),
			want: framesDuration(cbr441k128, 1),
		},
		{
			name: "id3v2 prefixed",
			data: append(id3v2(2048), cbr441k128.repeat(100)...),
			want: framesDuration(cbr441k128, 100),
		},
		{
			name: "id3v2 payload containing a false sync word",
			data: func() []byte {
				tag := id3v2(64)
				copy(tag[20:], []byte{0xFF, 0xFB, 0x92, 0xC0}) // looks like a frame, is not
				return append(tag, cbr441k128.repeat(50)...)
			}(),
			want: framesDuration(cbr441k128, 50),
		},
		{
			name: "id3v1 trailer excluded",
			data: append(cbr441k128.repeat(40), id3v1()...),
			want: framesDuration(cbr441k128, 40),
		},
		{
			name: "id3v2 and id3v1 around the audio",
			data: append(append(id3v2(512), cbr441k128.repeat(40)...), id3v1()...),
			want: framesDuration(cbr441k128, 40),
		},
		{
			name: "vbr xing frame count wins over frames present",
			data: append(xingFrame(stereo441k, 1000), stereo441k.repeat(3)...),
			want: framesDuration(stereo441k, 1000),
		},
		{
			name: "vbr xing on a mono stream",
			data: append(xingFrame(cbr441k128, 77), cbr441k128.repeat(2)...),
			want: framesDuration(cbr441k128, 77),
		},
		{
			name: "vbri frame count",
			data: append(vbriFrame(stereo441k, 250), stereo441k.repeat(2)...),
			want: framesDuration(stereo441k, 250),
		},
		{
			name: "mpeg2 layer III is 576 samples per frame",
			data: mpeg2L3.repeat(200),
			want: framesDuration(mpeg2L3, 200),
		},
		{
			name: "layer I is 384 samples per frame",
			data: layerI.repeat(64),
			want: framesDuration(layerI, 64),
		},
		{
			name: "layer II is 1152 samples per frame",
			data: layerII.repeat(64),
			want: framesDuration(layerII, 64),
		},
		{
			name: "leading garbage is skipped",
			data: append(bytes.Repeat([]byte{0x00, 0xFF, 0x13, 0xE0}, 64), cbr441k128.repeat(30)...),
			want: framesDuration(cbr441k128, 30),
		},
		{
			name: "garbage between frames resyncs without losing the tail",
			data: func() []byte {
				out := cbr441k128.repeat(10)
				out = append(out, bytes.Repeat([]byte{0xAB}, 700)...)
				return append(out, cbr441k128.repeat(10)...)
			}(),
			want: framesDuration(cbr441k128, 20),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Duration(tt.data)
			if err != nil {
				t.Fatalf("Duration() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Duration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDurationRejectsInputWithoutFrames(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "zero length slice", data: []byte{}},
		{name: "too short for a header", data: []byte{0xFF, 0xFB}},
		{name: "text", data: []byte("this is not audio, it is a sentence about audio")},
		{name: "zeros", data: bytes.Repeat([]byte{0x00}, 4096)},
		{name: "0xff run with no valid header", data: bytes.Repeat([]byte{0xFF}, 4096)},
		{name: "id3v2 tag only", data: id3v2(1024)},
		{
			name: "reserved mpeg version",
			data: bytes.Repeat([]byte{0xFF, 0xEB, 0x92, 0xC0}, 256),
		},
		{
			name: "reserved layer",
			data: bytes.Repeat([]byte{0xFF, 0xF9, 0x92, 0xC0}, 256),
		},
		{
			name: "bad bitrate index",
			data: bytes.Repeat([]byte{0xFF, 0xFB, 0xF2, 0xC0}, 256),
		},
		{
			name: "free bitrate index",
			data: bytes.Repeat([]byte{0xFF, 0xFB, 0x02, 0xC0}, 256),
		},
		{
			name: "reserved sample rate index",
			data: bytes.Repeat([]byte{0xFF, 0xFB, 0x9E, 0xC0}, 256),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Duration(tt.data)
			if err == nil {
				t.Fatalf("Duration() = %v, want error", got)
			}
		})
	}
}

func TestDurationDoesNotTrustAnIsolatedSyncWord(t *testing.T) {
	// A single well-formed header with nothing after it is real audio (the tail of
	// a file), but a header followed by a mismatched one is payload noise: the
	// scan must walk past it to the run of frames that agree with each other.
	noise := []byte{0xFF, 0xFB, 0x92, 0xC0}
	data := append(noise, bytes.Repeat([]byte{0x5A}, 64)...)
	data = append(data, cbr441k128.repeat(12)...)

	got, err := Duration(data)
	if err != nil {
		t.Fatalf("Duration() error = %v", err)
	}
	if want := framesDuration(cbr441k128, 12); got != want {
		t.Errorf("Duration() = %v, want %v", got, want)
	}
}

func TestDurationDoesNotMutateInput(t *testing.T) {
	data := append(id3v2(64), cbr441k128.repeat(8)...)
	before := bytes.Clone(data)

	if _, err := Duration(data); err != nil {
		t.Fatalf("Duration() error = %v", err)
	}
	if !bytes.Equal(data, before) {
		t.Error("Duration() mutated its input")
	}
}
