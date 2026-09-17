package audio

import (
	"encoding/binary"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordingsAndMix(t *testing.T) {
	m := synthesizer{rng: rand.New(rand.NewSource(1))}
	if e := m.load(); e != nil {
		t.Fatal(e)
	}
	for _, i := range []int{0, 3, 4} {
		if len(m.loops[i]) < rate*20*2 {
			t.Fatal("short recording", i)
		}
	}
	s := State{Mixer: true, Volumes: [5]int{35, 20, 20, 20, 20}}
	b := m.render(s, rate)
	nonzero := false
	for i := 0; i < len(b); i += 2 {
		if int16(binary.LittleEndian.Uint16(b[i:])) != 0 {
			nonzero = true
		}
	}
	if !nonzero {
		t.Fatal("silent mix")
	}
	m.render(State{}, rate*3)
	for _, g := range m.gains {
		if math.Abs(g) > 1e-8 {
			t.Fatal("gain did not fade", g)
		}
	}
}
func TestChimeEndsAndSilence(t *testing.T) {
	m := synthesizer{rng: rand.New(rand.NewSource(1)), chime: 1}
	m.render(State{}, rate*2)
	if m.chime != 0 {
		t.Fatal("chime repeated")
	}
	for _, v := range m.render(State{}, 882) {
		if v != 0 {
			t.Fatal("expected silence")
		}
	}
}
func TestMissingPlayerAndShutdown(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	e := New()
	e.Set(State{Radio: true})
	select {
	case v := <-e.Events():
		if !v.RadioFailed {
			t.Fatal(v)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no missing dependency error")
	}
	done := make(chan struct{})
	go func() { e.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown blocked")
	}
}

// Run the real subprocess/PCM path without requiring an audio device or mpv.
func TestPlayerHelper(t *testing.T) {
	if os.Getenv("LOFI_TEST_PLAYER") != "1" {
		return
	}
	b := make([]byte, 4096)
	if _, err := io.ReadFull(os.Stdin, b); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("LOFI_TEST_OUTPUT"), b, 0600); err != nil {
		os.Exit(3)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}
func TestPCMProcessLifecycle(t *testing.T) {
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec '" + strings.ReplaceAll(exe, "'", "'\\''") + "' -test.run=TestPlayerHelper -- \"$@\"\n"
	if err = os.WriteFile(filepath.Join(dir, "mpv"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "pcm.raw")
	t.Setenv("PATH", dir)
	t.Setenv("LOFI_TEST_PLAYER", "1")
	t.Setenv("LOFI_TEST_OUTPUT", output)
	e := New()
	defer e.Close()
	e.Set(State{Mixer: true, Volumes: [5]int{35, 20, 10, 10, 10}, Chime: 1})
	deadline := time.After(40 * time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case event := <-e.Events():
			if event.RadioFailed || event.MixerFailed {
				t.Fatal(event)
			}
		case <-deadline:
			t.Fatal("PCM player did not receive audio")
		case <-ticker.C:
			if b, err := os.ReadFile(output); err == nil && len(b) == 4096 {
				nonzero := false
				for _, v := range b {
					if v != 0 {
						nonzero = true
					}
				}
				if !nonzero {
					t.Fatal("silent PCM")
				}
				done := make(chan struct{})
				go func() { e.Close(); close(done) }()
				select {
				case <-done:
					return
				case <-time.After(3 * time.Second):
					t.Fatal("player not stopped")
				}
			}
		}
	}
}
