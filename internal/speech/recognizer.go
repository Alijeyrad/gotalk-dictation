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
	"os/exec"
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

// Recognizer performs speech-to-text using either the unofficial free Google API
// (default) or the official Google Cloud Speech API (when credentials are present).
type Recognizer struct {
	Language string
}

// Recognize transcribes audio from audioCh and returns the text.
// It automatically selects the free or cloud API based on credential availability.
func (r *Recognizer) Recognize(ctx context.Context, audioCh <-chan []byte) (string, error) {
	if HasCloudCredentials() {
		return r.recognizeCloud(ctx, audioCh)
	}
	return r.recognizeFree(ctx, audioCh)
}

// HasCloudCredentials reports whether Google Cloud credentials are available.
func HasCloudCredentials() bool {
	if _, ok := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS"); ok {
		return true
	}
	// Application Default Credentials from gcloud CLI
	home, _ := os.UserHomeDir()
	adc := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
	_, err := os.Stat(adc)
	return err == nil
}

// ---- Free (unofficial) API -----------------------------------------------

// recognizeFree buffers audio using VAD, then sends it to the unofficial API.
func (r *Recognizer) recognizeFree(ctx context.Context, audioCh <-chan []byte) (string, error) {
	pcm, err := bufferWithVAD(ctx, audioCh)
	if len(pcm) == 0 {
		return "", err // timeout/cancel or no speech
	}
	flac, ferr := pcmToFLAC(pcm)
	if ferr != nil {
		return "", fmt.Errorf("encoding audio: %w", ferr)
	}
	return postFreeAPI(ctx, flac, r.Language)
}

// pcmToFLAC converts raw 16-bit mono 16kHz PCM to FLAC using ffmpeg.
func pcmToFLAC(pcm []byte) ([]byte, error) {
	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-f", "s16le", "-ar", "16000", "-ac", "1",
		"-i", "pipe:0",
		"-f", "flac",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(pcm)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}
	return out, nil
}

// bufferWithVAD collects PCM audio, performing energy-based voice activity detection.
// Returns audio from speech start through end-of-phrase silence.
func bufferWithVAD(ctx context.Context, audioCh <-chan []byte) ([]byte, error) {
	const (
		calibChunks        = 8  // ~0.5 s of ambient calibration
		speechThresholdMul = 2.5
		minSpeechChunks    = 2  // debounce: N consecutive loud chunks = speech
		silenceEndChunks   = 8  // ~1 s of quiet after speech ends phrase
	)

	type state int
	const (
		calibrating state = iota
		waitingSpeech
		inSpeech
	)

	var (
		cur            state
		calibCount     int
		calibRMSSum    float64
		threshold      float64
		speechCount    int
		silenceCount   int
		preSpeech      [][]byte // small ring buffer kept before speech starts
		result         []byte
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
				calibRMSSum += rms
				if calibCount >= calibChunks {
					ambient := calibRMSSum / float64(calibCount)
					threshold = ambient * speechThresholdMul
					if threshold < 150 {
						threshold = 150
					}
					cur = waitingSpeech
				}

			case waitingSpeech:
				// Maintain a short pre-roll buffer so we don't clip the start of speech.
				preSpeech = append(preSpeech, chunk)
				if len(preSpeech) > 4 {
					preSpeech = preSpeech[1:]
				}
				if rms > threshold {
					speechCount++
					if speechCount >= minSpeechChunks {
						cur = inSpeech
						for _, c := range preSpeech {
							result = append(result, c...)
						}
						preSpeech = nil
						speechCount = 0
					}
				} else {
					speechCount = 0
				}

			case inSpeech:
				result = append(result, chunk...)
				if rms <= threshold {
					silenceCount++
					if silenceCount >= silenceEndChunks {
						return result, nil
					}
				} else {
					silenceCount = 0
				}
			}
		}
	}
}

// calcRMS computes Root Mean Square energy of 16-bit little-endian PCM samples.
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

// makeWAV wraps raw 16-bit mono 16kHz PCM in a WAV container.
func makeWAV(pcm []byte) []byte {
	var buf bytes.Buffer
	size := uint32(len(pcm))

	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, size+36) //nolint:errcheck
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))          //nolint:errcheck // chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))           //nolint:errcheck // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(1))           //nolint:errcheck // mono
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))  //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2)) //nolint:errcheck // byte rate
	binary.Write(&buf, binary.LittleEndian, uint16(2))           //nolint:errcheck // block align
	binary.Write(&buf, binary.LittleEndian, uint16(16))          //nolint:errcheck // bits per sample

	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, size) //nolint:errcheck
	buf.Write(pcm)

	return buf.Bytes()
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

// postFreeAPI sends FLAC audio to the unofficial Google Speech API and returns the transcript.
func postFreeAPI(ctx context.Context, wavData []byte, language string) (string, error) {
	key := os.Getenv("GOOGLE_API_KEY")
	if key == "" {
		key = defaultFreeKey
	}

	u, _ := url.Parse(freeAPIEndpoint)
	q := u.Query()
	q.Set("client", "chromium")
	q.Set("lang", language)
	q.Set("key", key)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(wavData))
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

	// Response is newline-delimited JSON; find the line with actual results.
	var transcript string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r freeAPIResponse
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		for _, result := range r.Result {
			if len(result.Alternative) > 0 {
				transcript = result.Alternative[0].Transcript
			}
		}
	}

	return transcript, nil
}

// ---- Official Google Cloud Speech API ------------------------------------

// recognizeCloud streams audio to the Google Cloud Speech-to-Text API.
// Requires credentials (GOOGLE_APPLICATION_CREDENTIALS or gcloud ADC).
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
