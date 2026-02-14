package speech

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// naiveCRC8 computes CRC-8 (poly 0x07) without using the package's table.
func naiveCRC8(data []byte) byte {
	var crc byte
	for _, b := range data {
		crc ^= b
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ 0x07
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// naiveCRC16 computes CRC-16 (poly 0x8005) without using the package's table.
func naiveCRC16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x8005
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func TestFlacUTF8Int(t *testing.T) {
	tests := []struct {
		name  string
		v     uint64
		want  []byte
	}{
		// Range 1: v < 0x80
		{"zero", 0x00, []byte{0x00}},
		{"ascii max", 0x7F, []byte{0x7F}},
		// Range 2: 0x80 <= v < 0x800
		{"0x80", 0x80, []byte{0xC2, 0x80}},
		{"0x7FF", 0x7FF, []byte{0xDF, 0xBF}},
		// Range 3: 0x800 <= v < 0x10000
		{"0x800", 0x800, []byte{0xE0, 0xA0, 0x80}},
		// Range 4: 0x10000 <= v < 0x200000
		{"0x10000", 0x10000, []byte{0xF0, 0x90, 0x80, 0x80}},
		// Range 5: v >= 0x200000
		{"0x200000", 0x200000, []byte{0xF8, 0x88, 0x80, 0x80, 0x80}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := flacUTF8Int(tc.v)
			if !bytes.Equal(got, tc.want) {
				t.Errorf("flacUTF8Int(0x%X) = %x, want %x", tc.v, got, tc.want)
			}
		})
	}
}

func TestFlacCRC8EmptyIsZero(t *testing.T) {
	if got := flacCRC8(nil); got != 0 {
		t.Errorf("flacCRC8(nil) = 0x%02X, want 0x00", got)
	}
	if got := flacCRC8([]byte{}); got != 0 {
		t.Errorf("flacCRC8([]) = 0x%02X, want 0x00", got)
	}
}

func TestFlacCRC8KnownValue(t *testing.T) {
	inputs := [][]byte{
		{0x01},
		{0xFF, 0xF8, 0x70, 0x00, 0x00},
		{0x01, 0x02, 0x03, 0x04},
	}
	for _, input := range inputs {
		want := naiveCRC8(input)
		got := flacCRC8(input)
		if got != want {
			t.Errorf("flacCRC8(%x) = 0x%02X, naiveCRC8 = 0x%02X", input, got, want)
		}
	}
}

func TestFlacCRC16EmptyIsZero(t *testing.T) {
	if got := flacCRC16(nil); got != 0 {
		t.Errorf("flacCRC16(nil) = 0x%04X, want 0x0000", got)
	}
	if got := flacCRC16([]byte{}); got != 0 {
		t.Errorf("flacCRC16([]) = 0x%04X, want 0x0000", got)
	}
}

func TestFlacCRC16KnownValue(t *testing.T) {
	inputs := [][]byte{
		{0x01},
		{0xFF, 0xF8, 0x70, 0x00, 0x00, 0x02, 0x12, 0x34},
		{0x01, 0x02, 0x03, 0x04},
	}
	for _, input := range inputs {
		want := naiveCRC16(input)
		got := flacCRC16(input)
		if got != want {
			t.Errorf("flacCRC16(%x) = 0x%04X, naiveCRC16 = 0x%04X", input, got, want)
		}
	}
}

func TestFlacStreamInfo(t *testing.T) {
	si := flacStreamInfo(1024)

	// Length must be 34
	if len(si) != 34 {
		t.Fatalf("flacStreamInfo len = %d, want 34", len(si))
	}

	// Min/max blocksize = 4096 = 0x1000
	minBS := binary.BigEndian.Uint16(si[0:2])
	maxBS := binary.BigEndian.Uint16(si[2:4])
	if minBS != flacBlockSize {
		t.Errorf("minBlockSize = %d, want %d", minBS, flacBlockSize)
	}
	if maxBS != flacBlockSize {
		t.Errorf("maxBlockSize = %d, want %d", maxBS, flacBlockSize)
	}

	// Verify sample rate (20-bit field) at bytes 10-12.
	// sr = 16000 = 0x003E80
	// si[10] = sr >> 12 = 0x03
	// si[11] = byte(sr >> 4) = 0xE8
	// si[12] upper nibble = sr & 0xF = 0x0
	if si[10] != 0x03 {
		t.Errorf("si[10] (sr hi) = 0x%02X, want 0x03", si[10])
	}
	if si[11] != 0xE8 {
		t.Errorf("si[11] (sr mid) = 0x%02X, want 0xE8", si[11])
	}
	if si[12]>>4 != 0x00 {
		t.Errorf("si[12] upper nibble (sr lo) = 0x%X, want 0x0", si[12]>>4)
	}

	// Verify bps=16 encoded as bpsM1=15 at si[13] upper nibble.
	if si[13]>>4 != 0x0F {
		t.Errorf("si[13] upper nibble (bpsM1) = 0x%X, want 0xF", si[13]>>4)
	}

	// Verify totalSamples=1024 in si[13] lower nibble + si[14:18].
	// ts=1024=0x0000_0000_0000_0400 (36 bits); upper 4 bits in si[13]&0xF = 0,
	// lower 32 bits as BigEndian uint32 at si[14:18] = 0x00000400.
	if si[13]&0x0F != 0x00 {
		t.Errorf("si[13] lower nibble (ts hi) = 0x%X, want 0x0", si[13]&0x0F)
	}
	tsLo := binary.BigEndian.Uint32(si[14:18])
	if tsLo != 1024 {
		t.Errorf("totalSamples (lower 32) = %d, want 1024", tsLo)
	}
}

func TestPCMToFLACNativeMagic(t *testing.T) {
	// 2 samples = 4 bytes of PCM
	pcm := []byte{0x00, 0x00, 0x00, 0x00}
	out := pcmToFLACNative(pcm)
	if len(out) < 4 {
		t.Fatalf("output too short: %d bytes", len(out))
	}
	if string(out[:4]) != "fLaC" {
		t.Errorf("magic = %q, want %q", string(out[:4]), "fLaC")
	}
}

func TestPCMToFLACNativeSampleByteSwap(t *testing.T) {
	// S16LE value 0x1234: bytes [0x34, 0x12].
	// FLAC VERBATIM stores big-endian: should see [0x12, 0x34] in subframe.
	pcm := []byte{0x34, 0x12}
	out := pcmToFLACNative(pcm)

	// Subframe type byte 0x02 followed by big-endian sample [0x12, 0x34].
	pattern := []byte{0x02, 0x12, 0x34}
	if !bytes.Contains(out, pattern) {
		t.Errorf("output does not contain big-endian subframe pattern %x\nfull output: %x", pattern, out)
	}
}

func TestPCMToFLACNativeSubframeTypeByte(t *testing.T) {
	// VERBATIM subframe header: high bit=0, type=0b000001, wasted_bits=0 → 0b00000010 = 0x02.
	pcm := make([]byte, 4)
	out := pcmToFLACNative(pcm)
	// 0x02 must appear somewhere after the header/streaminfo (42 bytes).
	if len(out) <= 42 {
		t.Fatalf("output too short: %d", len(out))
	}
	found := false
	for _, b := range out[42:] {
		if b == 0x02 {
			found = true
			break
		}
	}
	if !found {
		t.Error("subframe type byte 0x02 not found in frame data")
	}
}

func TestPCMToFLACNativeFrameCRC16(t *testing.T) {
	// Use 2 samples worth of PCM (fits in one partial frame).
	pcm := []byte{0x34, 0x12, 0x78, 0x56}
	out := pcmToFLACNative(pcm)

	// The FLAC output is: 4 (magic) + 4 (meta hdr) + 34 (streaminfo) = 42 bytes header.
	// Then frame bytes follow.
	frameData := out[42:]
	if len(frameData) < 3 {
		t.Fatalf("frame data too short: %d bytes", len(frameData))
	}

	// Frame format: <header><subframe>[CRC16 hi][CRC16 lo]
	// Strip last 2 bytes, recompute CRC16, compare with stored value.
	frameWithoutCRC := frameData[:len(frameData)-2]
	want := naiveCRC16(frameWithoutCRC)
	got := uint16(frameData[len(frameData)-2])<<8 | uint16(frameData[len(frameData)-1])
	if got != want {
		t.Errorf("frame CRC16 = 0x%04X, want 0x%04X", got, want)
	}
}

func TestPCMToFLACNativeFrameCRC8(t *testing.T) {
	// Use 2 samples. Verify CRC8 byte in frame header.
	pcm := []byte{0x00, 0x01, 0x00, 0x02}
	out := pcmToFLACNative(pcm)

	// Frame starts at byte 42.
	frameData := out[42:]

	// For a partial frame with frameNum=0:
	// hdr = [0xFF, 0xF8, bsCode<<4, 0x00, flacUTF8Int(0), blocksize-1 hi, blocksize-1 lo, CRC8]
	// flacUTF8Int(0) = [0x00] (1 byte)
	// nSamples = 2, blocksize-1 = 1 → [0x00, 0x01]
	// Total hdr bytes before CRC8 = 2+1+1+1+2 = 7 bytes
	if len(frameData) < 8 {
		t.Fatalf("frame data too short for CRC8 check: %d bytes", len(frameData))
	}
	hdrWithoutCRC := frameData[:7]
	wantCRC8 := naiveCRC8(hdrWithoutCRC)
	gotCRC8 := frameData[7]
	if gotCRC8 != wantCRC8 {
		t.Errorf("frame header CRC8 = 0x%02X, want 0x%02X", gotCRC8, wantCRC8)
	}
}

func TestPCMToFLACNativePartialLastFrame(t *testing.T) {
	// flacBlockSize*2 full samples + 100 extra bytes → two frames.
	// Second frame is partial, bsCode must be 0x07.
	pcm := make([]byte, flacBlockSize*2+100)
	out := pcmToFLACNative(pcm)

	// Find second frame: first frame is at byte 42.
	// First frame size (non-partial, frameNum=0):
	// hdr = [0xFF,0xF8, 0xC0, 0x00, flacUTF8Int(0)=0x00, CRC8] = 6 bytes
	// sub = 1 + flacBlockSize*2 bytes
	// CRC16 = 2 bytes
	// Total = 6 + 1 + flacBlockSize*2 + 2 = flacBlockSize*2 + 9 bytes
	frame1Size := flacBlockSize*2 + 9
	frame2Start := 42 + frame1Size
	if len(out) < frame2Start+3 {
		t.Fatalf("output too short for second frame: %d", len(out))
	}

	// Second frame starts with sync [0xFF, 0xF8].
	if out[frame2Start] != 0xFF || out[frame2Start+1] != 0xF8 {
		t.Errorf("second frame sync = [0x%02X, 0x%02X], want [0xFF, 0xF8]",
			out[frame2Start], out[frame2Start+1])
	}
	// Third byte: bsCode << 4; for partial frame bsCode=0x07 → 0x70.
	if out[frame2Start+2] != 0x70 {
		t.Errorf("second frame bsCode byte = 0x%02X, want 0x70 (bsCode=0x07)", out[frame2Start+2])
	}
}

func TestPCMToFLACNativeEmpty(t *testing.T) {
	// nil/empty input → valid fLaC header + STREAMINFO, no frames, no panic.
	out := pcmToFLACNative(nil)
	if len(out) == 0 {
		t.Fatal("expected non-empty output for nil input")
	}
	if string(out[:4]) != "fLaC" {
		t.Errorf("magic = %q, want %q", string(out[:4]), "fLaC")
	}
	// Should be exactly 42 bytes: 4 magic + 4 meta hdr + 34 streaminfo.
	if len(out) != 42 {
		t.Errorf("empty PCM output length = %d, want 42", len(out))
	}
}
