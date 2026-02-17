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
	defaultFreeKey  = "AIzaSyBOti4mM-6x9WDnZIjIeyEU21OpBXqWBgw"
	freeAPIEndpoint = "https://www.google.com/speech-api/v2/recognize"
	sampleRate      = 16000
)

type Recognizer struct {
	Language       string
	APIKey         string
	UseAdvancedAPI bool

	// SilenceChunks is consecutive silent chunks (~62 ms each) that end a phrase.
	SilenceChunks int

	// Sensitivity is the RMS threshold multiplier (lower = more sensitive).
	Sensitivity float64

	// SkipVAD buffers all incoming audio without voice-activity detection.
	// Use for push-to-talk mode where the key-release signals end-of-speech.
	SkipVAD bool

	// OnProcessing is called when VAD ends and the API call begins,
	// allowing the UI to switch from "Listening" to "Processing".
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

func (r *Recognizer) Recognize(ctx context.Context, audioCh <-chan []byte) (string, error) {
	if r.UseAdvancedAPI || HasCloudCredentials() {
		if r.OnProcessing != nil {
			r.OnProcessing()
		}
		return r.recognizeCloud(ctx, audioCh)
	}
	return r.recognizeFree(ctx, audioCh)
}

func HasCloudCredentials() bool {
	if _, ok := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS"); ok {
		return true
	}
	home, _ := os.UserHomeDir()
	adc := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
	_, err := os.Stat(adc)
	return err == nil
}

func (r *Recognizer) recognizeFree(ctx context.Context, audioCh <-chan []byte) (string, error) {
	var pcm []byte
	var err error
	if r.SkipVAD {
		pcm = bufferAll(ctx, audioCh)
	} else {
		pcm, err = r.bufferWithVAD(ctx, audioCh)
	}

	if r.OnProcessing != nil {
		r.OnProcessing()
	}

	if len(pcm) == 0 {
		return "", err
	}

	return r.postFreeAPI(ctx, pcmToFLACNative(pcm))
}

// bufferAll reads every chunk from audioCh until the channel is closed or ctx
// is cancelled. Used for PTT mode where key-release signals end-of-speech.
func bufferAll(ctx context.Context, audioCh <-chan []byte) []byte {
	var result []byte
	for {
		select {
		case chunk, ok := <-audioCh:
			if !ok {
				return result
			}
			result = append(result, chunk...)
		case <-ctx.Done():
			return result
		}
	}
}

func (r *Recognizer) bufferWithVAD(ctx context.Context, audioCh <-chan []byte) ([]byte, error) {
	const (
		calibChunks     = 4 // ~0.5 s ambient calibration
		minSpeechChunks = 2 // consecutive loud chunks to confirm speech onset
		preRollLen      = 4 // chunks buffered before onset to avoid clipping
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

func makeWAV(pcm []byte) []byte {
	var buf bytes.Buffer
	size := uint32(len(pcm))
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, size+36)       //nolint:errcheck
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))    //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint16(1))     //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint16(1))     //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))   //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2)) //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint16(2))     //nolint:errcheck
	binary.Write(&buf, binary.LittleEndian, uint16(16))    //nolint:errcheck
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, size)          //nolint:errcheck
	buf.Write(pcm)
	return buf.Bytes()
}

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
