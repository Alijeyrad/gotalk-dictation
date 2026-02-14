package speech

// Pure Go FLAC verbatim (uncompressed) encoder.
// Supports: 16-bit signed PCM, 16 kHz sample rate, mono channel.
// Implements only what is needed for the Google Speech API (FLAC subset 0).
// No external tools required.

import (
	"bytes"
	"encoding/binary"
)

const (
	flacSampleRate = 16000
	flacBPS        = 16
	flacBlockSize  = 4096 // samples per frame (last frame may be smaller)
)

// pcmToFLACNative encodes raw S16LE PCM (16 kHz, mono) to FLAC without any
// external tools by using the VERBATIM (pass-through) subframe type.
func pcmToFLACNative(pcm []byte) []byte {
	nSamples := int64(len(pcm) / 2)

	var out bytes.Buffer
	out.WriteString("fLaC")

	// METADATA_BLOCK_HEADER: last-block=1, type=STREAMINFO(0), length=34
	out.Write([]byte{0x80, 0x00, 0x00, 0x22})
	si := flacStreamInfo(nSamples)
	out.Write(si[:])

	for frameNum := 0; int64(frameNum)*flacBlockSize < nSamples; frameNum++ {
		start := frameNum * flacBlockSize * 2 // byte offset in pcm
		end := start + flacBlockSize*2
		if end > len(pcm) {
			end = len(pcm)
		}
		out.Write(flacEncodeFrame(frameNum, nSamples, pcm[start:end]))
	}
	return out.Bytes()
}

// flacStreamInfo returns the 34-byte STREAMINFO block payload.
func flacStreamInfo(totalSamples int64) [34]byte {
	var si [34]byte
	binary.BigEndian.PutUint16(si[0:], flacBlockSize) // min blocksize
	binary.BigEndian.PutUint16(si[2:], flacBlockSize) // max blocksize
	// bytes 4-9: min/max framesize = 0 (unknown)

	// Bytes 10-17 pack: sample_rate(20 bits) | channels-1(3) | bps-1(5) | total_samples(36)
	sr := uint32(flacSampleRate) // 16000 = 0x3E80
	bpsM1 := uint32(flacBPS - 1) // 15
	ts := uint64(totalSamples)

	si[10] = byte(sr >> 12)                                          // sr[19:12]
	si[11] = byte(sr >> 4)                                           // sr[11:4]
	si[12] = byte((sr&0xF)<<4) | byte(0<<1) | byte(bpsM1>>4)        // sr[3:0] | ch | bps[4]
	si[13] = byte(bpsM1&0xF)<<4 | byte(ts>>32)                      // bps[3:0] | ts[35:32]
	binary.BigEndian.PutUint32(si[14:], uint32(ts))      // ts[31:0]
	// bytes 18-33: MD5 signature (zeros = not computed, valid per spec)
	return si
}

// flacEncodeFrame encodes one FLAC audio frame using the VERBATIM subframe type.
func flacEncodeFrame(frameNum int, totalSamples int64, pcm []byte) []byte {
	nSamples := len(pcm) / 2
	isLastPartial := int64((frameNum+1)*flacBlockSize) > totalSamples && nSamples != flacBlockSize

	// ---- Frame Header ----
	var hdr bytes.Buffer

	// Sync code (14 bits=0x3FFE) + reserved(0) + blocking_strategy(0=fixed)
	// = bytes 0xFF 0xF8
	hdr.Write([]byte{0xFF, 0xF8})

	// Block size code (4 bits) | sample rate code (4 bits = 0000 = from streaminfo)
	var bsCode byte
	if isLastPartial {
		bsCode = 0x07 // 16-bit block size - 1 follows in header
	} else {
		bsCode = 0x0C // 4096 samples
	}
	hdr.WriteByte(bsCode<<4 | 0x00)

	// Channel assignment (4 bits = 0000 mono) | sample size (3 bits = 000 from streaminfo) | reserved(0)
	hdr.WriteByte(0x00)

	// Frame number: UTF-8-coded unsigned integer
	hdr.Write(flacUTF8Int(uint64(frameNum)))

	// Variable block size for last partial frame
	if isLastPartial {
		var buf [2]byte
		binary.BigEndian.PutUint16(buf[:], uint16(nSamples-1))
		hdr.Write(buf[:])
	}

	// CRC-8 of header bytes so far
	hdr.WriteByte(flacCRC8(hdr.Bytes()))

	// ---- Subframe: VERBATIM (type 0b000001) ----
	var sub bytes.Buffer
	sub.WriteByte(0x02) // 0 | 000001 | 0 = zero-bit | VERBATIM | no-wasted-bits

	// Raw 16-bit samples, stored MSB-first (big-endian, not the little-endian arecord output)
	for i := 0; i < len(pcm); i += 2 {
		sub.WriteByte(pcm[i+1]) // high byte
		sub.WriteByte(pcm[i])   // low byte
	}

	// ---- Frame Footer: CRC-16 over header+subframe ----
	frameData := append(hdr.Bytes(), sub.Bytes()...)
	crc16 := flacCRC16(frameData)
	frameData = append(frameData, byte(crc16>>8), byte(crc16))
	return frameData
}

// flacUTF8Int encodes a non-negative integer using FLAC's UTF-8-like variable-length coding.
func flacUTF8Int(v uint64) []byte {
	switch {
	case v < 0x80:
		return []byte{byte(v)}
	case v < 0x800:
		return []byte{0xC0 | byte(v>>6), 0x80 | byte(v&0x3F)}
	case v < 0x10000:
		return []byte{0xE0 | byte(v>>12), 0x80 | byte((v>>6)&0x3F), 0x80 | byte(v&0x3F)}
	case v < 0x200000:
		return []byte{0xF0 | byte(v>>18), 0x80 | byte((v>>12)&0x3F), 0x80 | byte((v>>6)&0x3F), 0x80 | byte(v&0x3F)}
	default:
		return []byte{
			0xF8 | byte(v>>24),
			0x80 | byte((v>>18)&0x3F),
			0x80 | byte((v>>12)&0x3F),
			0x80 | byte((v>>6)&0x3F),
			0x80 | byte(v&0x3F),
		}
	}
}

// ---- CRC tables (initialized once) ----

var (
	crc8Table  [256]byte
	crc16Table [256]uint16
)

func init() {
	// CRC-8: polynomial x^8 + x^2 + x + 1 = 0x07
	for i := range crc8Table {
		v := byte(i)
		for j := 0; j < 8; j++ {
			if v&0x80 != 0 {
				v = (v << 1) ^ 0x07
			} else {
				v <<= 1
			}
		}
		crc8Table[i] = v
	}
	// CRC-16: polynomial x^16 + x^15 + x^2 + 1 = 0x8005
	for i := range crc16Table {
		v := uint16(i) << 8
		for j := 0; j < 8; j++ {
			if v&0x8000 != 0 {
				v = (v << 1) ^ 0x8005
			} else {
				v <<= 1
			}
		}
		crc16Table[i] = v
	}
}

func flacCRC8(data []byte) byte {
	var crc byte
	for _, b := range data {
		crc = crc8Table[crc^b]
	}
	return crc
}

func flacCRC16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc = crc16Table[byte(crc>>8)^b] ^ (crc << 8)
	}
	return crc
}
