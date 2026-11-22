package alert

import (
	"encoding/binary"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// The tone is what a person actually hears when a timer runs out, so its
// container format is checked byte by byte rather than "it isn't empty":
// a WAV that a player silently refuses to open would be indistinguishable
// from a muted machine.
func TestToneIsAValidPCMWAV(t *testing.T) {
	tone := Tone()
	if len(tone) < 44 {
		t.Fatalf("tone is %d bytes, too short to contain a WAV header", len(tone))
	}

	assertString(t, "RIFF", tone[0:4])
	riffSize := binary.LittleEndian.Uint32(tone[4:8])
	if want := uint32(len(tone) - 8); riffSize != want {
		t.Errorf("RIFF size = %d, want %d (file length minus the 8-byte RIFF/WAVE prefix)", riffSize, want)
	}
	assertString(t, "WAVE", tone[8:12])
	assertString(t, "fmt ", tone[12:16])

	if got := binary.LittleEndian.Uint32(tone[16:20]); got != 16 {
		t.Errorf("fmt chunk size = %d, want 16 (canonical PCM)", got)
	}
	if got := binary.LittleEndian.Uint16(tone[20:22]); got != 1 {
		t.Errorf("audio format = %d, want 1 (uncompressed PCM)", got)
	}
	if got := binary.LittleEndian.Uint16(tone[22:24]); got != 1 {
		t.Errorf("channels = %d, want 1", got)
	}
	if got := binary.LittleEndian.Uint32(tone[24:28]); got != sampleRate {
		t.Errorf("sample rate = %d, want %d", got, sampleRate)
	}
	if got := binary.LittleEndian.Uint32(tone[28:32]); got != sampleRate*2 {
		t.Errorf("byte rate = %d, want %d", got, sampleRate*2)
	}
	if got := binary.LittleEndian.Uint16(tone[32:34]); got != 2 {
		t.Errorf("block align = %d, want 2", got)
	}
	if got := binary.LittleEndian.Uint16(tone[34:36]); got != 16 {
		t.Errorf("bits per sample = %d, want 16", got)
	}

	assertString(t, "data", tone[36:40])
	dataSize := binary.LittleEndian.Uint32(tone[40:44])
	if want := uint32(len(tone) - 44); dataSize != want {
		t.Errorf("data chunk size = %d, want %d", dataSize, want)
	}

	wantSamples := 0
	for _, b := range tonePattern {
		wantSamples += (sampleRate / 1000) * b.ms
	}
	if got := int(dataSize) / 2; got != wantSamples {
		t.Errorf("tone has %d samples, want %d (the pattern's total duration at %dHz)", got, wantSamples, sampleRate)
	}
	if wantSamples < sampleRate/2 {
		t.Errorf("tone is only %d samples long; an alert shorter than half a second is easy to miss", wantSamples)
	}
}

// A tone of all zeroes would be a valid WAV and completely inaudible.
func TestToneActuallyMakesSound(t *testing.T) {
	tone := Tone()
	var peak int
	nonZero := 0
	for i := 44; i+1 < len(tone); i += 2 {
		v := int(int16(binary.LittleEndian.Uint16(tone[i : i+2])))
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
		if v != 0 {
			nonZero++
		}
	}

	if peak == 0 {
		t.Fatal("tone is silent (every sample is zero)")
	}
	// Roughly a third of the pattern is intentional silence between beeps.
	if nonZero < len(tone)/8 {
		t.Errorf("only %d non-zero samples; expected the beeps to dominate", nonZero)
	}
	if peak >= 32767 {
		t.Errorf("peak sample %d is at full scale, which clips and sounds like distortion", peak)
	}
}

func TestToneIsDeterministic(t *testing.T) {
	first := Tone()
	second := Tone()
	if len(first) != len(second) {
		t.Fatalf("tone length changed between calls: %d then %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("tone byte %d changed between calls: %d then %d", i, first[i], second[i])
		}
	}
}

// Play hands the tone to the platform player on its own goroutine, and
// collapses bursts so an alarm and a timer finishing in the same tick
// don't stack into noise.
func TestPlayPlaysOnceAndDebouncesBursts(t *testing.T) {
	var calls int32
	played := make(chan string, 4)
	restore := playFileFunc
	playFileFunc = func(path string) error {
		atomic.AddInt32(&calls, 1)
		played <- path
		return nil
	}
	defer func() {
		playFileFunc = restore
		playMu.Lock()
		lastPlayedAt = time.Time{}
		playMu.Unlock()
	}()
	playMu.Lock()
	lastPlayedAt = time.Time{}
	playMu.Unlock()

	Play()
	select {
	case <-played:
	case <-time.After(5 * time.Second):
		t.Fatal("Play never reached the audio backend within 5s")
	}

	// Immediately after, three more alarms land in the same tick.
	Play()
	Play()
	Play()

	select {
	case <-played:
		t.Fatal("a second tone played inside the debounce window; overlapping alarms would be noise")
	case <-time.After(150 * time.Millisecond):
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("audio backend called %d times, want 1", got)
	}
}

// The path handed to the platform player must be a real, readable WAV
// file: Windows' PlaySound is given the path, not the bytes, and reads
// it from the audio thread after Play has already returned.
func TestPlayWritesAReadableToneFile(t *testing.T) {
	var gotPath string
	restore := playFileFunc
	playFileFunc = func(path string) error {
		gotPath = path
		return nil
	}
	defer func() {
		playFileFunc = restore
		playMu.Lock()
		lastPlayedAt = time.Time{}
		playMu.Unlock()
	}()
	playMu.Lock()
	lastPlayedAt = time.Time{}
	playMu.Unlock()

	// Stop the audio thread from reading a stale script's leftover file: the
	// tone file is written by the same package, so assert it exists after Play.
	Play()
	deadline := time.Now().Add(5 * time.Second)
	for gotPath == "" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if gotPath == "" {
		t.Fatal("Play never handed a path to the audio backend within 5s")
	}

	data, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("reading the tone file Play handed to the backend: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("tone file is empty")
	}
	if string(data[:4]) != "RIFF" {
		t.Errorf("tone file does not start with RIFF (got %q)", string(data[:4]))
	}
}

func assertString(t *testing.T, want string, got []byte) {
	t.Helper()
	if string(got) != want {
		t.Errorf("chunk tag = %q, want %q", string(got), want)
	}
}
