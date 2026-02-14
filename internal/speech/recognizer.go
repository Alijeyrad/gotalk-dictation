package speech

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	speechapi "cloud.google.com/go/speech/apiv1"
	speechpb "cloud.google.com/go/speech/apiv1/speechpb"
)

const (
	// defaultFreeKey is the public Chromium browser key used by speech_recognition.
	defaultFreeKey  = "AIzaSyBOti4mM-6x9WDnZIjIeyEU21OpBXqWBgw"
	freeAPIEndpoint = "https://www.google.com/speech-api/v2/recognize"
	sampleRate      = 16000
)

// Recognizer performs speech-to-text using either the unofficial free Google
// API (default) or the official Google Cloud Speech API.
type Recognizer struct {
	Language string

	// APIKey overrides the built-in Chromium key for the free API.
	APIKey string

	// UseAdvancedAPI forces use of the Google Cloud Speech API.
	UseAdvancedAPI bool

	// SilenceChunks is the number of consecutive silent chunks that end a phrase.
	// Each chunk is ~62 ms. Default 12 (~0.75 s).
	SilenceChunks int

	// Sensitivity is the RMS threshold multiplier (lower = more sensitive).
	Sensitivity float64

	// OnProcessing is called when VAD ends and the API call is about to begin.
	// Use it to switch the UI from "Listening" to "Processing".
	OnProcessing func()
}

func (r *Recognizer) silenceChunks() int {
	if r.SilenceChunks > 0 {
		return r.SilenceChunks
	}
	return 12
}

func (r *Recognizer) sensitivity() float64 {
	if r.Sensitivity > 0 {
		return r.Sensitivity
	}
	return 2.5
}

// Recognize transcribes audio from audioCh and returns the transcript.
func (r *Recognizer) Recognize(ctx context.Context, audioCh <-chan []byte) (string, error) {
	if r.UseAdvancedAPI || HasCloudCredentials() {
		if r.OnProcessing != nil {
			r.OnProcessing()
		}
		return r.recognizeCloud(ctx, audioCh)
	}
	return r.recognizeFree(ctx, audioCh)
}

// HasCloudCredentials reports whether Google Cloud credentials are configured.
func HasCloudCredentials() bool {
	if _, ok := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS"); ok {
		return true
	}
	home, _ := os.UserHomeDir()
	adc := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
	_, err := os.Stat(adc)
	return err == nil
}

// ---- Free (unofficial) API -----------------------------------------------

func (r *Recognizer) recognizeFree(ctx context.Context, audioCh <-chan []byte) (string, error) {
	// VAD phase — UI stays in "Listening" state.
	pcm, err := r.bufferWithVAD(ctx, audioCh)

	// Switch UI to "Processing" now that we have audio to send.
	if r.OnProcessing != nil {
		r.OnProcessing()
	}

	if len(pcm) == 0 {
		return "", err
	}

	// Encode PCM → FLAC using our native encoder (no ffmpeg required).
	flacData := pcmToFLACNative(pcm)
	return r.postFreeAPI(ctx, flacData)
}

// bufferWithVAD collects PCM audio with energy-based Voice Activity Detection.
// It calibrates against ambient noise, then captures from first speech onset
// through the end-of-phrase silence.
func (r *Recognizer) bufferWithVAD(ctx context.Context, audioCh <-chan []byte) ([]byte, error) {
	const (
		calibChunks     = 8 // ~0.5 s ambient calibration
		minSpeechChunks = 2 // consecutive loud chunks to start capturing
		preRollLen      = 4 // chunks kept before speech onset (prevent clipping)
	)

	silenceEndChunks := r.silenceChunks()
	thresholdMul := r.sensitivity()

	type phase int
	const (
		calibrating phase = iota
		waitingSpeech
		inSpeech
	)

	var (
		cur         phase
		calibCount  int
		calibSum    float64
		threshold   float64
		speechCount int
		silCount    int
		preRoll     [][]byte
		result      []byte
	)

	for {
		select {
		case <-ctx.Done():
			return result, ctx.Err()

		case chunk, ok := <-audioCh:
			if !ok {
				return result, nil
			}

			rms := calcRMS(chunk)

			switch cur {
			case calibrating:
				calibCount++
				calibSum += rms
				if calibCount >= calibChunks {
					ambient := calibSum / float64(calibCount)
					threshold = ambient * thresholdMul
					if threshold < 150 {
						threshold = 150
					}
					cur = waitingSpeech
				}

			case waitingSpeech:
				preRoll = append(preRoll, chunk)
				if len(preRoll) > preRollLen {
					preRoll = preRoll[1:]
				}
				if rms > threshold {
					speechCount++
					if speechCount >= minSpeechChunks {
						cur = inSpeech
						for _, c := range preRoll {
							result = append(result, c...)
						}
						preRoll = nil
						speechCount = 0
					}
				} else {
					speechCount = 0
				}

			case inSpeech:
				result = append(result, chunk...)
				if rms <= threshold {
					silCount++
					if silCount >= silenceEndChunks {
						return result, nil
					}
				} else {
					silCount = 0
				}
			}
		}
	}
}

// calcRMS computes Root Mean Square energy of 16-bit little-endian PCM.
func calcRMS(pcm []byte) float64 {
	n := len(pcm) / 2
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < len(pcm)-1; i += 2 {
		s := float64(int16(binary.LittleEndian.Uint16(pcm[i : i+2])))
		sum += s * s
	}
	return math.Sqrt(sum / float64(n))
}

type freeAPIResponse struct {
	Result []struct {
		Alternative []struct {
			Transcript string  `json:"transcript"`
			Confidence float64 `json:"confidence"`
		} `json:"alternative"`
		Final bool `json:"final"`
	} `json:"result"`
}

func (r *Recognizer) postFreeAPI(ctx context.Context, flacData []byte) (string, error) {
	key := r.APIKey
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	if key == "" {
		key = defaultFreeKey
	}

	u, _ := url.Parse(freeAPIEndpoint)
	q := u.Query()
	q.Set("client", "chromium")
	q.Set("lang", r.Language)
	q.Set("key", key)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(flacData))
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", fmt.Sprintf("audio/x-flac; rate=%d", sampleRate))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned HTTP %d", resp.StatusCode)
	}

	var transcript string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var res freeAPIResponse
		if err := json.Unmarshal(line, &res); err != nil {
			continue
		}
		for _, result := range res.Result {
			if len(result.Alternative) > 0 {
				transcript = result.Alternative[0].Transcript
			}
		}
	}
	return transcript, nil
}

// makeWAV wraps raw 16-bit mono 16kHz PCM in a WAV container.
// Used by the Cloud API path which accepts LINEAR16.
func makeWAV(pcm []byte) []byte {
	var buf bytes.Buffer
	size := uint32(len(pcm))
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, size+36)  //nolint:errcheck
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))         //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint16(1))          //nolint:errcheck // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(1))          //nolint:errcheck // mono
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate)) //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2)) //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint16(2))          //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint16(16))         //nolint:errcheck
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, size) //nolint:errcheck
	buf.Write(pcm)
	return buf.Bytes()
}

// ---- Official Google Cloud Speech API ------------------------------------

func (r *Recognizer) recognizeCloud(ctx context.Context, audioCh <-chan []byte) (string, error) {
	client, err := speechapi.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("creating speech client: %w", err)
	}
	defer client.Close()

	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		return "", fmt.Errorf("creating stream: %w", err)
	}

	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz: sampleRate,
					LanguageCode:    r.Language,
				},
				SingleUtterance: true,
				InterimResults:  false,
			},
		},
	}); err != nil {
		return "", fmt.Errorf("sending config: %w", err)
	}

	go func() {
		defer stream.CloseSend()
		for {
			select {
			case chunk, ok := <-audioCh:
				if !ok {
					return
				}
				if err := stream.Send(&speechpb.StreamingRecognizeRequest{
					StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
						AudioContent: chunk,
					},
				}); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	var finalText string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if ctx.Err() != nil {
				return finalText, ctx.Err()
			}
			return finalText, fmt.Errorf("receiving: %w", err)
		}
		for _, result := range resp.Results {
			if result.IsFinal && len(result.Alternatives) > 0 {
				finalText += result.Alternatives[0].Transcript
			}
		}
	}
	return finalText, nil
}
