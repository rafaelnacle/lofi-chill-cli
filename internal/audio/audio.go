// Package audio mixes local ambience and controls mpv radio playback.
// An Engine owns its worker and player processes; callers must close it.
package audio

import (
	"bytes"
	"context"
	"embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/hajimehoshi/go-mp3"
)

//go:embed assets/*
var assets embed.FS

// Channels returns a fresh array of display names in mixer order.
func Channels() [5]string {
	return [5]string{"Rainfall", "Brown noise", "Ocean hush", "Birdsong", "Fireplace"}
}

// Station pairs a display name with its YouTube page URL, not a direct audio stream.
type Station struct {
	Name string // Name is the label shown in the TUI.
	URL  string // URL identifies the remote YouTube page.
}

// Stations returns the radio catalog by value so callers cannot change shared state.
func Stations() [4]Station {
	return [4]Station{
		{Name: "Lofi Girl · study", URL: "https://www.youtube.com/watch?v=rFZHOHl-L8A"},
		{Name: "steezyasfuck · hip hop", URL: "https://www.youtube.com/watch?v=rPjez8z61rI"},
		{Name: "Lofi Girl · synthwave", URL: "https://www.youtube.com/watch?v=4xDzrJKXOOY"},
		{Name: "Lofi Girl · sleep/chill", URL: "https://www.youtube.com/watch?v=JD-kMIpDfnY"},
	}
}

const rate = 44100

// State is the desired playback snapshot. Mixer and Radio enable their outputs.
// Volumes contains channel percentages in Channels order; RadioVolume is also 0–100.
// Station indexes Stations. Increment Chime to request one completion sound.
type State struct {
	Mixer        bool
	Volumes      [5]int
	Radio        bool
	Station      int
	RadioVolume  int
	Chime        int
	RadioRequest int // RadioRequest identifies the latest radio start/stop or station change.
	MixerRequest int // MixerRequest identifies the latest mixer start/stop.
}

// Playback identifies the player's observed output state.
type Playback string

const (
	// Connecting means playback has not started or is waiting for data.
	Connecting Playback = "Conectando"
	// Playing means mpv reports active playback with a valid media position.
	Playing Playback = "Tocando"
)

// Event reports observed playback state or a recoverable playback failure. Message is safe to show in the TUI.
// RadioFailed and MixerFailed identify which requested output should be stopped.
type Event struct {
	Message      string
	RadioFailed  bool
	MixerFailed  bool
	RadioStatus  Playback // RadioStatus is empty when the event concerns another output.
	MixerStatus  Playback // MixerStatus is empty when the event concerns another output.
	RadioRequest int      // RadioRequest associates the event with the initiating request.
	MixerRequest int      // MixerRequest associates the event with the initiating request.
}

// Engine serializes audio work in an owned goroutine.
// Use New to construct it and Close to release it. Set and Close are safe
// to call concurrently; drain Events to observe playback failures.
type Engine struct {
	updates chan State
	events  chan Event
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once
}

// New starts an idle audio worker without starting playback.
// The caller must call Close even when no audio was requested. Initialization
// and playback errors are reported asynchronously through Events.
func New() *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{
		updates: make(chan State, 1),
		events:  make(chan Event, 16),
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	go e.run(ctx)
	return e
}

// Events returns playback failures until the worker stops and closes the channel.
// The channel is receive-only; callers do not own its lifetime.
func (e *Engine) Events() <-chan Event { return e.events }

// Set submits the latest desired state without blocking on playback.
// Queued snapshots may be coalesced. Volumes must be 0–100 and Station must
// index Stations. Set after Close has no effect.
func (e *Engine) Set(s State) {
	select {
	case e.updates <- s:
	default:
		select {
		case <-e.updates:
		default:
		}
		select {
		case e.updates <- s:
		default:
		}
	}
}

// Close cancels playback and waits for the worker and player processes to stop.
// It is safe to call more than once. Events closes when the worker exits.
func (e *Engine) Close() {
	e.once.Do(e.cancel)
	<-e.done
}

// report queues a failure without stalling audio when the event buffer is full.
func (e *Engine) report(v Event) {
	select {
	case e.events <- v:
	default:
	}
}

type process struct {
	cmd    *exec.Cmd
	done   chan error
	input  io.WriteCloser
	status <-chan Playback
}

// start launches mpv in its own process group, optionally accepting PCM on stdin.
// The owner must consume done or call stop; cancellation also kills helper processes.
func start(ctx context.Context, args []string, pipe bool, socket string) (*process, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, "mpv", args...)
	// mpv may start yt-dlp. Cancel the entire private group to avoid orphan helpers.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 2 * time.Second
	p := &process{cmd: cmd, done: make(chan error, 1)}
	var err error
	if pipe {
		p.input, err = cmd.StdinPipe()
		if err != nil {
			cancel()
			return nil, err
		}
	}
	if err = cmd.Start(); err != nil {
		if p.input != nil {
			p.input.Close()
		}
		cancel()
		return nil, err
	}
	status := make(chan Playback, 1)
	observed := make(chan struct{})
	p.status = status
	go func() { defer close(observed); observePlayback(ctx, socket, status) }()
	go func() {
		err := cmd.Wait()
		cancel()
		<-observed
		// The player may have exited while an extractor child is still running.
		_ = cmd.Cancel()
		p.done <- err
	}()
	return p, nil
}

// stop terminates a player group and waits for it to be reaped.
// A nil player is harmless; a non-nil player must be stopped only once.
func (p *process) stop() {
	if p == nil {
		return
	}
	if p.input != nil {
		p.input.Close()
	}
	_ = p.cmd.Cancel()
	<-p.done
}

// radioVolume sends a bounded IPC volume request; failures can be retried
// while mpv is creating its socket.
func radioVolume(socket string, v int) error {
	conn, err := net.DialTimeout("unix", socket, 30*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		return err
	}
	return json.NewEncoder(conn).Encode(map[string]any{"command": []any{"set_property", "volume", v}})
}

// startRadio checks the extractor dependency and starts the selected station.
func startRadio(ctx context.Context, socket string, state State) (*process, error) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return nil, fmt.Errorf("rádio: instale yt-dlp no PATH: %w", err)
	}
	args := []string{
		"--no-config",
		"--no-video",
		"--no-terminal",
		"--ytdl=yes",
		"--network-timeout=15",
		"--ytdl-format=bestaudio/best",
		"--script-opts=ytdl_hook-ytdl_path=yt-dlp",
		"--input-ipc-server=" + socket,
		fmt.Sprintf("--volume=%d", state.RadioVolume),
		"--", Stations()[state.Station].URL,
	}
	player, err := start(ctx, args, false, socket)
	if err != nil {
		return nil, fmt.Errorf("rádio: não foi possível iniciar mpv: %w", err)
	}
	return player, nil
}

// startPCM opens an mpv output accepting the synthesizer's stereo PCM format.
func startPCM(ctx context.Context, socket string) (*process, error) {
	return start(ctx, []string{
		"--no-config",
		"--no-video",
		"--no-terminal",
		"--cache=no",
		"--demuxer=rawaudio",
		"--demuxer-rawaudio-rate=44100",
		"--demuxer-rawaudio-channels=stereo",
		"--demuxer-rawaudio-format=s16le",
		"--audio-buffer=0.1",
		"--input-ipc-server=" + socket,
		"-",
	}, true, socket)
}

// run owns mutable playback state until cancellation and releases all resources on exit.
func (e *Engine) run(ctx context.Context) {
	defer close(e.done)
	defer close(e.events)
	dir, err := os.MkdirTemp("", "lofi-chill-")
	if err != nil {
		e.report(Event{Message: err.Error(), MixerFailed: true, RadioFailed: true})
		return
	}
	defer os.RemoveAll(dir)
	var pcm, radio *process
	defer func() {
		pcm.stop()
		radio.stop()
	}()
	var s State
	var mix synthesizer
	mix.rng = rand.New(rand.NewSource(1))
	var loaded bool
	var failed bool
	var lastChime int
	var socket = filepath.Join(dir, "radio.sock")
	pcmSocket := filepath.Join(dir, "mixer.sock")
	report := func(event Event) {
		event.RadioRequest = s.RadioRequest
		event.MixerRequest = s.MixerRequest
		e.report(event)
	}
	var volume = -1
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case next := <-e.updates:
			if !next.Mixer {
				failed = false
			}
			if next.Chime != lastChime {
				mix.chime = 1
				lastChime = next.Chime
				failed = false
			}
			old := s
			s = next
			if next.Radio && (!old.Radio || next.Station != old.Station) {
				radio.stop()
				radio = nil
				_ = os.Remove(socket)
				radio, err = startRadio(ctx, socket, next)
				if err != nil {
					report(Event{Message: err.Error(), RadioFailed: true})
					next.Radio = false
				}
				volume = next.RadioVolume
			} else if !next.Radio && radio != nil {
				radio.stop()
				radio = nil
			}
			s = next
		case <-tick.C:
		}
		if radio != nil {
			select {
			case <-radio.done:
				radio = nil
				s.Radio = false
				report(Event{Message: "Rádio encerrado ou indisponível. Pressione p para tentar novamente.", RadioFailed: true})
			default:
			}
			if radio != nil && s.RadioVolume != volume {
				if radioVolume(socket, s.RadioVolume) == nil {
					volume = s.RadioVolume
				}
			}
		}
		if pcm != nil {
			select {
			case <-pcm.done:
				pcm.input.Close()
				pcm = nil
				mix.chime = 0
				failed = true
				report(Event{Message: "Saída de áudio encerrada. Verifique mpv/dispositivo e tente novamente.", MixerFailed: true})
			default:
			}
		}
		if (s.Mixer || mix.chime > 0) && pcm == nil && !failed {
			if !loaded {
				if err = mix.load(); err != nil {
					failed = true
					report(Event{Message: err.Error(), MixerFailed: true})
					continue
				}
				loaded = true
			}
			_ = os.Remove(pcmSocket)
			pcm, err = startPCM(ctx, pcmSocket)
			if err != nil {
				failed = true
				mix.chime = 0
				report(Event{Message: "Áudio: instale mpv no PATH para mixer e aviso.", MixerFailed: true})
				continue
			}
		}
		if radio != nil && s.Radio {
			select {
			case status := <-radio.status:
				report(Event{RadioStatus: status})
			default:
			}
		}
		if pcm != nil && s.Mixer {
			select {
			case status := <-pcm.status:
				report(Event{MixerStatus: status})
			default:
			}
		}
		if pcm != nil {
			b := mix.render(s, 882)
			if _, err = pcm.input.Write(b); err != nil {
				pcm.stop()
				pcm = nil
				mix.chime = 0
				failed = true
				report(Event{Message: "Falha na saída de áudio. Verifique o dispositivo e tente novamente.", MixerFailed: true})
			}
		}
	}
}

type synthesizer struct {
	loops     [5][]float64
	positions [5]int
	gains     [5]float64
	noise     [2]float64
	rng       *rand.Rand
	sample    int64
	chime     int
}

// load decodes the embedded recordings and crossfades their loop boundaries.
// It must finish before the recordings are rendered and is not concurrency-safe.
func (m *synthesizer) load() error {
	for i, name := range map[int]string{0: "rain", 3: "birds", 4: "fire"} {
		b, err := assets.ReadFile("assets/" + name + ".mp3")
		if err != nil {
			return err
		}
		d, err := mp3.NewDecoder(bytes.NewReader(b))
		if err != nil {
			return err
		}
		if d.SampleRate() != rate {
			return fmt.Errorf("gravação %s: sample rate incompatível", name)
		}
		raw, err := io.ReadAll(d)
		if err != nil {
			return err
		}
		samples := make([]float64, len(raw)/2)
		for j := range samples {
			samples[j] = float64(int16(binary.LittleEndian.Uint16(raw[j*2:]))) / 32768
		}
		if len(samples) < rate {
			return fmt.Errorf("gravação %s incompleta", name)
		}
		// Overlap the final 50ms with the beginning, then loop after that head.
		fade := rate / 20 * 2
		for j := 0; j < fade; j++ {
			a := float64(j) / float64(fade)
			samples[len(samples)-fade+j] = samples[len(samples)-fade+j]*(1-a) + samples[j]*a
		}
		m.loops[i] = samples[fade:]
	}
	return nil
}

// render advances the mixer and returns frames of 44.1 kHz stereo signed 16-bit PCM.
// It smooths gain changes and consumes a pending chime once. Calls must be serialized.
func (m *synthesizer) render(s State, frames int) []byte {
	out := make([]byte, frames*4)
	for frame := 0; frame < frames; frame++ {
		for i := range m.gains {
			target := 0.0
			if s.Mixer {
				target = float64(s.Volumes[i]) / 100
			}
			m.gains[i] += (target - m.gains[i]) / (0.12 * rate)
		}
		ch := 0.0
		if m.chime > 0 {
			t := float64(m.chime-1) / rate
			for i, f := range []float64{523.25, 659.25} {
				u := t - float64(i)*0.24
				if u >= 0 && u < 1.3 {
					gain := 0.07 * math.Exp(-5.4*math.Max(0, u-0.035))
					if u < 0.035 {
						gain = 0.07 * u / 0.035
					}
					ch += math.Sin(2*math.Pi*f*u) * gain
				}
			}
			m.chime++
			if t >= 1.54 {
				m.chime = 0
			}
		}
		for side := 0; side < 2; side++ {
			white := m.rng.Float64()*2 - 1
			m.noise[side] = (m.noise[side] + 0.02*white) / 1.02
			wave := m.noise[side] * 2 * (0.55 + 0.45*math.Sin(2*math.Pi*0.12*float64(m.sample)/rate))
			v := ch + m.noise[side]*1.75*m.gains[1] + wave*m.gains[2]
			for _, i := range []int{0, 3, 4} {
				if len(m.loops[i]) > 0 {
					v += m.loops[i][(m.positions[i]+side)%len(m.loops[i])] * m.gains[i]
				}
			}
			v = math.Max(-1, math.Min(1, v))
			binary.LittleEndian.PutUint16(out[frame*4+side*2:], uint16(int16(v*32767)))
		}
		for _, i := range []int{0, 3, 4} {
			if len(m.loops[i]) > 0 {
				m.positions[i] = (m.positions[i] + 2) % len(m.loops[i])
			}
		}
		m.sample++
	}
	return out
}
