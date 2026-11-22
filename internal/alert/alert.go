// Package alert owns STELLAREI's own audible alarm tone: the sound a
// person hears when a timer or alarm comes due.
//
// It exists as a SEPARATE path from internal/notify on purpose. The
// operating system's notification pipeline is not a reliable way to
// make a noise:
//
//   - Windows "Do Not Disturb"/Focus Assist silences the toast's own
//     <audio> element completely while still accepting the toast.
//   - A person can switch notifications off for the app, or turn the
//     notification banner off while leaving the app running.
//   - macOS drops the banner (and its sound) entirely if
//     RequestNotificationAuthorization was denied.
//
// None of those are things STELLAREI can detect or fix, and none of
// them should mean "time is up" passes unnoticed. So the tone is
// synthesised here and handed straight to the platform's audio layer,
// with nothing in between that a notification setting can mute. The
// toast (and the in-app banner) remain as the visual channels.
package alert

import (
	"bytes"
	"encoding/binary"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Tone is a short, deliberately attention-grabbing three-beep pattern
// (two rising notes, then a higher, longer one) written to a temporary
// WAV file once per process and replayed from there. 16-bit mono PCM at
// 44.1kHz is the one format every platform's player accepts without any
// conversion: winmm's PlaySound reads it directly, and so do afplay,
// paplay, aplay and ffplay on the other side.
const sampleRate = 44100

// beep is one note in Tone's pattern.
type beep struct {
	freqHz  float64
	ms      int
	silence bool
}

// tonePattern is the alarm: ding, ding, ding-DONG. Kept short (under a
// second) so it reads as an alert rather than a ringtone, and kept to
// pure tones so the WAV is ~75KB rather than an embedded asset.
var tonePattern = []beep{
	{freqHz: 880, ms: 180},
	{silence: true, ms: 90},
	{freqHz: 880, ms: 180},
	{silence: true, ms: 90},
	{freqHz: 1046.5, ms: 320},
}

// toneOnce/toneBytes cache the generated file. The bytes are pure
// function of tonePattern, so generating them once per process (not once
// per alarm) keeps a burst of simultaneous alarms cheap.
var (
	toneDataOnce sync.Once
	toneData     []byte
)

// Tone returns the alarm tone as a complete 16-bit mono PCM WAV file.
// Deterministic and allocation-cheap, so it is directly unit-testable
// (see tone_test.go) without touching the audio device.
func Tone() []byte {
	toneDataOnce.Do(func() {
		toneData = encodeWAV(renderPattern(tonePattern))
	})
	return toneData
}

// renderPattern turns the note pattern into raw little-endian 16-bit
// samples, fading each tone in and out over a few milliseconds - a note
// that starts or stops at full amplitude clicks audibly, which sounds
// like a bug rather than an alarm.
func renderPattern(pattern []beep) []byte {
	samplesPerMs := sampleRate / 1000
	const fadeMs = 5
	const amplitude = 0.38 // leaves headroom; 1.0 would clip once several tones overlap in the mix

	out := make([]byte, 0, 4096)
	for _, b := range pattern {
		count := samplesPerMs * b.ms
		for i := 0; i < count; i++ {
			var v float64
			if !b.silence {
				t := float64(i) / float64(sampleRate)
				v = amplitude * math.Sin(2*math.Pi*b.freqHz*t)
				// Linear fade over the first and last fadeMs of the note.
				if fade := fadeMs * samplesPerMs; fade > 0 {
					if i < fade {
						v *= float64(i) / float64(fade)
					} else if rem := count - i; rem < fade {
						v *= float64(rem) / float64(fade)
					}
				}
			}
			sample := int16(v * 32767)
			out = binary.LittleEndian.AppendUint16(out, uint16(sample))
		}
	}
	return out
}

// encodeWAV wraps raw 16-bit mono samples in a canonical 44-byte WAV
// header.
func encodeWAV(samples []byte) []byte {
	const (
		channels      = 1
		bitsPerSample = 16
		headerSize    = 44
	)
	byteRate := sampleRate * channels * bitsPerSample / 8

	buf := bytes.NewBuffer(make([]byte, 0, headerSize+len(samples)))
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+len(samples))) // file size minus the first 8 bytes
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16)) // PCM fmt chunk size
	binary.Write(buf, binary.LittleEndian, uint16(1))  // format 1 = uncompressed PCM
	binary.Write(buf, binary.LittleEndian, uint16(channels))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(buf, binary.LittleEndian, uint16(channels*bitsPerSample/8)) // block align
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(len(samples)))
	buf.Write(samples)
	return buf.Bytes()
}

// toneFilePath is where the tone is written before playback. It is
// written once and then left in place for the lifetime of the process:
// on Windows PlaySound is handed the path (not the bytes), so the file
// has to still exist when the audio thread opens it.
var toneFilePath = filepath.Join(os.TempDir(), "stellarei-alarm.wav")

// minInterval stops a burst of events (an alarm and a Pomodoro phase
// finishing in the same tick, say) from stacking tones on top of each
// other into noise.
const minInterval = 700 * time.Millisecond

var (
	playMu       sync.Mutex
	lastPlayedAt time.Time
)

// playFileFunc is the seam the tests swap out; production always points
// at the platform backend selected by the build tags (play_windows.go,
// play_darwin.go, play_linux.go, play_other.go).
var playFileFunc = playFile

// Play sounds the alarm tone. It never blocks the caller - playback
// happens on its own goroutine - and it never returns an error: a
// machine with no working audio output must not stop a timer from
// completing or an alarm from being marked fired. A failure is logged
// (and therefore lands in the log file, see main.go) so it is
// diagnosable rather than silent.
func Play() {
	playMu.Lock()
	if !lastPlayedAt.IsZero() && time.Since(lastPlayedAt) < minInterval {
		playMu.Unlock()
		return
	}
	lastPlayedAt = time.Now()
	playMu.Unlock()

	go func() {
		path, err := ensureToneFile()
		if err != nil {
			log.Printf("alert: preparing alarm tone: %v", err)
			return
		}
		if err := playFileFunc(path); err != nil {
			log.Printf("alert: playing alarm tone: %v", err)
		}
	}()
}

var toneFileOnce sync.Once

// ensureToneFile writes the tone to toneFilePath the first time it is
// called and returns the path.
func ensureToneFile() (string, error) {
	var err error
	toneFileOnce.Do(func() {
		err = os.WriteFile(toneFilePath, Tone(), 0o600)
	})
	if err != nil {
		// Let a later call retry rather than caching a failure forever.
		toneFileOnce = sync.Once{}
		return "", err
	}
	return toneFilePath, nil
}
