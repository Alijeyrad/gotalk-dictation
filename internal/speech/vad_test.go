package speech

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"testing"
)

// makePCMSilence creates n S16LE samples all zero (RMS=0).
func makePCMSilence(n int) []byte {
	return make([]byte, n*2)
}

// makePCMLoud creates n S16LE samples all equal to int16(rms), giving RMS=|rms|.
func makePCMLoud(rms float64, n int) []byte {
	pcm := make([]byte, n*2)
	v := int16(rms)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(v))
	}
	return pcm
}

// makePCMValue creates n S16LE samples all equal to v.
func makePCMValue(v int16, n int) []byte {
	pcm := make([]byte, n*2)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(v))
	}
	return pcm
}

// feedChan sends all chunks to a buffered channel, closes it, and returns it.
func feedChan(chunks [][]byte) <-chan []byte {
	ch := make(chan []byte, len(chunks)+1)
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch
}

func TestCalcRMSZero(t *testing.T) {
	if got := calcRMS(nil); got != 0 {
		t.Errorf("calcRMS(nil) = %f, want 0", got)
	}
	if got := calcRMS(makePCMSilence(10)); got != 0 {
		t.Errorf("calcRMS(silence) = %f, want 0", got)
	}
}

func TestCalcRMSKnownValue(t *testing.T) {
	// Two S16LE samples of value 1000: PCM = [0xE8, 0x03, 0xE8, 0x03].
	pcm := []byte{0xE8, 0x03, 0xE8, 0x03}
	got := calcRMS(pcm)
	want := 1000.0
	if math.Abs(got-want) > 0.001 {
		t.Errorf("calcRMS = %f, want %f", got, want)
	}
}

func TestCalcRMSNegativeSample(t *testing.T) {
	// S16LE sample of value -1000: RMS is always positive.
	pcm := makePCMValue(-1000, 1)
	got := calcRMS(pcm)
	want := 1000.0
	if math.Abs(got-want) > 0.001 {
		t.Errorf("calcRMS(-1000 sample) = %f, want %f", got, want)
	}
}

func TestCalcRMSOddByteCount(t *testing.T) {
	// One byte → n = len(pcm)/2 = 0 → returns 0.
	if got := calcRMS([]byte{0xAB}); got != 0 {
		t.Errorf("calcRMS(1 byte) = %f, want 0", got)
	}
}

// newTestRecognizer creates a Recognizer configured for controlled testing.
// calibChunks=4, minSpeechChunks=2, preRollLen=4 are hardcoded in bufferWithVAD.
func newTestRecognizer(silenceChunks int) *Recognizer {
	return &Recognizer{
		SilenceChunks: silenceChunks,
		Sensitivity:   2.5,
	}
}

func TestVADFullPhase(t *testing.T) {
	// calibChunks=4, minSpeechChunks=2, preRollLen=4 (from bufferWithVAD constants)
	// SilenceChunks=3 so 3 silent chunks in inSpeech ends the phrase.
	r := newTestRecognizer(3)

	const samplesPerChunk = 50 // = 100 bytes per chunk
	silence := makePCMSilence(samplesPerChunk)
	loud := makePCMLoud(1000, samplesPerChunk) // RMS=1000 >> threshold=150

	var chunks [][]byte
	// Phase 1: 4 calibration chunks (silence, not in result).
	for i := 0; i < 4; i++ {
		chunks = append(chunks, silence)
	}
	// Phase 2 (waitingSpeech): 4 preRoll silence, then 2 loud → onset.
	for i := 0; i < 4; i++ {
		chunks = append(chunks, silence)
	}
	chunks = append(chunks, loud) // speechCount=1
	chunks = append(chunks, loud) // speechCount=2 → onset

	// Phase 3 (inSpeech): 5 loud + 3 silence → end.
	for i := 0; i < 5; i++ {
		chunks = append(chunks, loud)
	}
	for i := 0; i < 3; i++ {
		chunks = append(chunks, silence)
	}

	result, err := r.bufferWithVAD(context.Background(), feedChan(chunks))
	if err != nil {
		t.Fatalf("bufferWithVAD unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result after full speech phase")
	}
}

func TestVADThresholdFloor(t *testing.T) {
	// Ambient RMS=5, Sensitivity=2.5 → computed threshold = 12.5, floored to 150.
	// Chunks at RMS=100 must NOT trigger onset.
	r := newTestRecognizer(3)
	r.Sensitivity = 2.5

	const samplesPerChunk = 50
	calibChunk := makePCMValue(5, samplesPerChunk)  // RMS=5 → ambient=5
	belowFloor := makePCMLoud(100, samplesPerChunk) // RMS=100 < threshold=150

	var chunks [][]byte
	for i := 0; i < 4; i++ {
		chunks = append(chunks, calibChunk)
	}
	// Send many below-floor chunks; channel closes before onset.
	for i := 0; i < 8; i++ {
		chunks = append(chunks, belowFloor)
	}

	result, err := r.bufferWithVAD(context.Background(), feedChan(chunks))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result (no onset), got %d bytes", len(result))
	}
}

func TestVADPreRollIncluded(t *testing.T) {
	// Verify that preRoll chunks end up in the result after onset.
	// Use a distinct marker for the preRoll silence chunks (RMS=1, well below 150).
	r := newTestRecognizer(3)

	const samplesPerChunk = 50
	calibSilence := makePCMSilence(samplesPerChunk) // calibration chunks
	// preRoll marker: int16(1) samples → RMS=1 << 150
	markerSilence := makePCMValue(1, samplesPerChunk)
	loud := makePCMLoud(1000, samplesPerChunk)

	var chunks [][]byte
	// Phase 1: 4 calibration silence.
	for i := 0; i < 4; i++ {
		chunks = append(chunks, calibSilence)
	}
	// Phase 2 (waitingSpeech): 4 marker-silence preRoll chunks, then 2 loud → onset.
	// After onset, preRoll contains the last preRollLen(=4) chunks:
	// the 2 trailing marker-silence + 2 loud. So 2 marker-silence chunks make it in.
	for i := 0; i < 4; i++ {
		chunks = append(chunks, markerSilence)
	}
	chunks = append(chunks, loud)
	chunks = append(chunks, loud)
	// End with enough silence.
	for i := 0; i < 3; i++ {
		chunks = append(chunks, loud)
	}
	for i := 0; i < 3; i++ {
		chunks = append(chunks, calibSilence)
	}

	result, err := r.bufferWithVAD(context.Background(), feedChan(chunks))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	// Result should start with markerSilence bytes (from preRoll).
	// preRoll at onset time = [markerSilence[2], markerSilence[3], loud[0], loud[1]]
	// (the first 2 of the 4 marker-silence chunks were trimmed off).
	chunkBytes := samplesPerChunk * 2
	if len(result) < chunkBytes {
		t.Fatalf("result too short: %d bytes", len(result))
	}
	// The first chunk in result should be markerSilence (trimmed preRoll starts with it).
	if !bytes.Equal(result[:chunkBytes], markerSilence) {
		t.Error("result does not start with preRoll marker-silence bytes")
	}
}

func TestVADSilenceCountReset(t *testing.T) {
	// In inSpeech: 2 silence + 1 loud (resets silCount) + 3 silence → end.
	// SilenceChunks=3: 2 silence alone would NOT end it; after reset, 3 silence DO end it.
	r := newTestRecognizer(3)

	const samplesPerChunk = 50
	silence := makePCMSilence(samplesPerChunk)
	loud := makePCMLoud(1000, samplesPerChunk)

	var chunks [][]byte
	// Calibration.
	for i := 0; i < 4; i++ {
		chunks = append(chunks, silence)
	}
	// Onset: 2 consecutive loud.
	for i := 0; i < 6; i++ {
		chunks = append(chunks, silence) // preRoll
	}
	chunks = append(chunks, loud)
	chunks = append(chunks, loud)
	// inSpeech: 2 silence + 1 loud (reset) + 3 silence (end).
	chunks = append(chunks, silence) // silCount=1
	chunks = append(chunks, silence) // silCount=2
	chunks = append(chunks, loud)    // silCount=0 (reset)
	chunks = append(chunks, silence) // silCount=1
	chunks = append(chunks, silence) // silCount=2
	chunks = append(chunks, silence) // silCount=3 → end

	result, err := r.bufferWithVAD(context.Background(), feedChan(chunks))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestVADContextCancel(t *testing.T) {
	r := newTestRecognizer(3)

	ctx, cancel := context.WithCancel(context.Background())

	// Use an unbuffered channel; cancel context before any chunk arrives.
	ch := make(chan []byte)
	cancel() // cancel immediately

	_, err := r.bufferWithVAD(ctx, ch)
	if err == nil {
		t.Error("expected context.Canceled error, got nil")
	}
}

func TestVADChannelClose(t *testing.T) {
	// Close channel before onset → returns (result, nil) with empty result.
	r := newTestRecognizer(3)

	const samplesPerChunk = 50
	silence := makePCMSilence(samplesPerChunk)

	var chunks [][]byte
	// Only calibration chunks; channel closes after.
	for i := 0; i < 4; i++ {
		chunks = append(chunks, silence)
	}

	result, err := r.bufferWithVAD(context.Background(), feedChan(chunks))
	if err != nil {
		t.Errorf("unexpected error on channel close: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result on channel close, got %d bytes", len(result))
	}
}

func TestVADDefaultValues(t *testing.T) {
	r := &Recognizer{SilenceChunks: 0, Sensitivity: 0}
	if got := r.silenceChunks(); got != 12 {
		t.Errorf("silenceChunks() = %d, want 12", got)
	}
	if got := r.sensitivity(); got != 2.5 {
		t.Errorf("sensitivity() = %f, want 2.5", got)
	}
}
