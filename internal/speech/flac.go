package speech

import (
	"bytes"
	"encoding/binary"
)

const (
	flacSampleRate = 16000
	flacBPS        = 16
	flacBlockSize  = 4096 // samples per frame (last frame may be smaller)
)

func pcmToFLACNative(pcm []byte) []byte {
	nSamples := int64(len(pcm) / 2)

	var out bytes.Buffer
	out.WriteString("fLaC")
	out.Write([]byte{0x80, 0x00, 0x00, 0x22}) // METADATA_BLOCK_HEADER: last-block=1, STREAMINFO, length=34
	si := flacStreamInfo(nSamples)
	out.Write(si[:])

	for frameNum := 0; int64(frameNum)*flacBlockSize < nSamples; frameNum++ {
		start := frameNum * flacBlockSize * 2
		end := start + flacBlockSize*2
		if end > len(pcm) {
			end = len(pcm)
		}
		out.Write(flacEncodeFrame(frameNum, nSamples, pcm[start:end]))
	}
	return out.Bytes()
}

func flacStreamInfo(totalSamples int64) [34]byte {
	var si [34]byte
	binary.BigEndian.PutUint16(si[0:], flacBlockSize) // min blocksize
	binary.BigEndian.PutUint16(si[2:], flacBlockSize) // max blocksize
	// bytes 4-9: min/max framesize = 0 (unknown)

	// Bytes 10-17: sample_rate(20 bits) | channels-1(3) | bps-1(5) | total_samples(36)
	sr := uint32(flacSampleRate)
	bpsM1 := uint32(flacBPS - 1)
	ts := uint64(totalSamples)

	si[10] = byte(sr >> 12)
	si[11] = byte(sr >> 4)
	si[12] = byte((sr&0xF)<<4) | byte(0<<1) | byte(bpsM1>>4)
	si[13] = byte(bpsM1&0xF)<<4 | byte(ts>>32)
	binary.BigEndian.PutUint32(si[14:], uint32(ts))
	// bytes 18-33: MD5 signature (zeros = not computed, valid per spec)
	return si
}

func flacEncodeFrame(frameNum int, totalSamples int64, pcm []byte) []byte {
	nSamples := len(pcm) / 2
	isLastPartial := int64((frameNum+1)*flacBlockSize) > totalSamples && nSamples != flacBlockSize

	var hdr bytes.Buffer

	hdr.Write([]byte{0xFF, 0xF8}) // sync code + blocking_strategy=fixed

	var bsCode byte
	if isLastPartial {
		bsCode = 0x07 // 16-bit block size - 1 follows in header
	} else {
		bsCode = 0x0C // 4096 samples
	}
	hdr.WriteByte(bsCode << 4) // block size code | sample rate code (0=from streaminfo)

	hdr.WriteByte(0x00) // channel=mono | sample size=from streaminfo | reserved

	hdr.Write(flacUTF8Int(uint64(frameNum)))

	if isLastPartial {
		var buf [2]byte
		binary.BigEndian.PutUint16(buf[:], uint16(nSamples-1))
		hdr.Write(buf[:])
	}

	hdr.WriteByte(flacCRC8(hdr.Bytes()))

	var sub bytes.Buffer
	sub.WriteByte(0x02) // subframe type: VERBATIM (0b000001), no wasted bits

	// PCM is S16LE; FLAC requires big-endian samples.
	for i := 0; i < len(pcm); i += 2 {
		sub.WriteByte(pcm[i+1])
		sub.WriteByte(pcm[i])
	}

	frameData := append(hdr.Bytes(), sub.Bytes()...)
	crc16 := flacCRC16(frameData)
	return append(frameData, byte(crc16>>8), byte(crc16))
}

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

var (
	crc8Table  [256]byte
	crc16Table [256]uint16
)

func init() {
	for i := range crc8Table { // CRC-8: poly 0x07
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
	for i := range crc16Table { // CRC-16: poly 0x8005
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
