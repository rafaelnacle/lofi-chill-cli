// Package ui presents the Pomodoro, radio, and mixer through Bubble Tea.
package ui

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"lofi-chill/internal/audio"
	"lofi-chill/internal/config"
	"lofi-chill/internal/timer"
)

const purple = lipgloss.Color("141")
const pink = lipgloss.Color("212")
const cyan = lipgloss.Color("87")

var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var accent = lipgloss.NewStyle().Foreground(cyan).Bold(true)

type tick time.Time
type saved struct{ err error }
type persistence struct {
	mu     sync.Mutex
	closed bool
}

// Model owns TUI state and implements tea.Model.
// Bubble Tea must serialize Update calls; background commands return messages
// rather than mutate Model. The caller owns the audio engine passed to New.
type Model struct {
	writes                       *persistence
	helpOffset                   int
	timerRow, radioRow           int
	radioStatus, mixerStatus     string
	timer                        timer.Timer
	cfg                          config.Config
	path                         string
	engine                       *audio.Engine
	sound                        audio.State
	width, height, page, channel int
	focus, help, dirty, saving   bool
	notice                       string
	persist                      bool
}

// New builds a stopped interface from c and the caller-owned engine.
// A non-nil warning is displayed and disables persistence to preserve an unreadable
// or unsupported configuration file. path is used for subsequent preference saves.
func New(c config.Config, path string, engine *audio.Engine, warning error) Model {
	m := Model{
		writes:      &persistence{},
		timer:       timer.New(time.Now(), c.Durations, c.Day, c.Completed),
		cfg:         c,
		path:        path,
		engine:      engine,
		width:       80,
		height:      24,
		persist:     warning == nil,
		notice:      "Uma coisa de cada vez. Seu espaço está pronto.",
		radioStatus: "Parado", mixerStatus: "Pausado",
		timerRow: 2, radioRow: 2,
	}
	m.sound.Station = c.Station
	m.sound.RadioVolume = c.RadioVolume
	for i, k := range config.Channels() {
		m.sound.Volumes[i] = c.Volumes[k]
	}
	if warning != nil {
		m.notice = "Preferências não carregadas; arquivo preservado: " + warning.Error()
	}
	return m
}

// pulse schedules one timer observation; Update schedules the following pulse.
func pulse() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tick(t) })
}

// listen waits for one audio failure or worker shutdown without blocking Update.
func (m Model) listen() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.engine.Events()
		if !ok {
			return nil
		}
		return event
	}
}

// Init starts timer observations and audio-event listening through Bubble Tea commands.
func (m Model) Init() tea.Cmd { return tea.Batch(pulse(), m.listen()) }

// syncConfig copies persistent preferences and counts, excluding active sessions.
func (m *Model) syncConfig() {
	m.cfg.Durations = m.timer.Durations
	m.cfg.Day = m.timer.Day
	m.cfg.Completed = m.timer.Completed
	m.cfg.Station = m.sound.Station
	m.cfg.RadioVolume = m.sound.RadioVolume
	for i, k := range config.Channels() {
		m.cfg.Volumes[k] = m.sound.Volumes[i]
	}
}

// save snapshots dirty preferences and schedules at most one asynchronous write.
// The shared lock prevents a pending command from overwriting the final exit save.
func (m *Model) save() tea.Cmd {
	if !m.persist || !m.dirty || m.saving {
		return nil
	}
	m.syncConfig()
	c := m.cfg
	c.Volumes = map[string]int{}
	for k, v := range m.cfg.Volumes {
		c.Volumes[k] = v
	}
	path := m.path
	m.dirty = false
	m.saving = true
	writer := m.writes
	return func() tea.Msg {
		writer.mu.Lock()
		defer writer.mu.Unlock()
		if writer.closed {
			return saved{nil}
		}
		return saved{config.Save(path, c)}
	}
}

// Update applies one keyboard, clock, resize, or background-result message.
// It returns the next model and commands for work outside the event loop.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	before, day, count := m.timer.Completion, m.timer.Day, m.timer.Completed
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
	case tick:
		m.timer.Tick(time.Time(v))
		cmds = append(cmds, pulse())
	case saved:
		m.saving = false
		if v.err != nil {
			m.notice = "Falha ao salvar preferências: " + v.err.Error()
			m.persist = false
		}
	case audio.Event:
		radioEvent := v.RadioRequest == m.sound.RadioRequest && m.sound.Radio
		mixerEvent := v.MixerRequest == m.sound.MixerRequest && m.sound.Mixer
		if radioEvent && v.RadioFailed {
			m.sound.Radio = false
			m.radioStatus = "Falhou"
			m.notice = v.Message
		}
		if mixerEvent && v.MixerFailed {
			m.sound.Mixer = false
			m.mixerStatus = "Falhou"
			m.notice = v.Message
		}
		if !v.RadioFailed && radioEvent && v.RadioStatus != "" {
			if v.RadioStatus == audio.Playing && m.radioStatus != string(audio.Playing) {
				m.notice = "Rádio tocando."
			}
			m.radioStatus = string(v.RadioStatus)
		}
		if !v.MixerFailed && mixerEvent && v.MixerStatus != "" {
			if v.MixerStatus == audio.Playing && m.mixerStatus != string(audio.Playing) {
				m.notice = "Sons ambientes prontos. Ajuste os volumes com ←/→."
			}
			m.mixerStatus = string(v.MixerStatus)
		}
		// A failed chime still needs feedback when the mixer is paused.
		if v.MixerFailed && v.MixerRequest == m.sound.MixerRequest && !mixerEvent {
			m.notice = v.Message
		}
		m.sendSound()
		cmds = append(cmds, m.listen())
	case tea.KeyMsg:
		key := v.String()
		now := time.Now()
		if key == "ctrl+c" || key == "q" {
			m.syncConfig()
			return m, tea.Quit
		}
		if key == "?" {
			m.help = !m.help
			return m, nil
		}
		if key == "esc" {
			if m.help {
				m.help = false
			} else {
				m.focus = false
			}
			return m, nil
		}
		if m.help && key != " " {
			if key == "down" || key == "j" {
				m.helpOffset = min(max(0, len(helpLines())-m.bodyHeight()), m.helpOffset+1)
			}
			if key == "up" || key == "k" {
				m.helpOffset = max(0, m.helpOffset-1)
			}
			return m, nil
		}
		switch key {
		case " ":
			m.timer.Toggle(now)
		case "1", "2", "3":
			m.timer.Switch(timer.Mode(key[0]-'1'), now)
		case "r":
			m.timer.Reset(now)
			m.notice = "Sessão resetada. Pronto quando você estiver."
		case "+", "=", "-":
			delta := 1
			if key == "-" {
				delta = -1
			}
			m.timer.SetDuration(m.timer.Mode, m.timer.Durations[m.timer.Mode]+delta)
			m.dirty = true
		case "tab":
			m.page = (m.page + 1) % 3
			m.focus = false
		case "shift+tab":
			m.page = (m.page + 2) % 3
			m.focus = false
		case "f":
			m.focus = !m.focus
			if m.focus {
				m.page = 0
			}
		case "m":
			m.toggleMixer()
		case "p":
			m.toggleRadio()
		case "[":
			m.changeStation(-1)
			m.dirty = true
		case "]":
			m.changeStation(1)
			m.dirty = true
		case ",":
			m.sound.RadioVolume = max(0, m.sound.RadioVolume-5)
			m.dirty = true
		case ".":
			m.sound.RadioVolume = min(100, m.sound.RadioVolume+5)
			m.dirty = true
		case "up", "k":
			m.selectItem(-1)
		case "down", "j":
			m.selectItem(1)
		case "left", "h":
			m.adjustItem(-1, now)
		case "right", "l":
			m.adjustItem(1, now)
		case "enter":
			m.activateItem(now)
		case "c":
			m.cfg.Chime = !m.cfg.Chime
			m.dirty = true
		case "t":
			m.sound.Chime++
			m.notice = "Teste do aviso sonoro."
		}
		m.sendSound()
	}
	if m.timer.Completion != before {
		m.notice = "Sessão concluída. " + m.timer.Mode.String() + " pronta para iniciar."
		if m.cfg.Chime {
			m.sound.Chime++
			m.sendSound()
		}
	}
	if day != m.timer.Day || count != m.timer.Completed {
		m.dirty = true
	}
	cmds = append(cmds, m.save())
	return m, tea.Batch(cmds...)
}

// SaveOnExit writes final preferences and prevents older queued saves from replacing them.
// It returns storage errors and does nothing when persistence was disabled.
// Call it after the Bubble Tea program has stopped.
func (m Model) SaveOnExit() error {
	if !m.persist {
		return nil
	}
	m.writes.mu.Lock()
	defer m.writes.mu.Unlock()
	m.writes.closed = true
	m.syncConfig()
	return config.Save(m.path, m.cfg)
}

// sendSound submits an audio snapshot without coupling layout tests to a player.
func (m Model) sendSound() {
	if m.engine != nil {
		m.engine.Set(m.sound)
	}
}

// selectItem moves selection within the active panel without changing playback.
func (m *Model) selectItem(delta int) {
	switch m.page {
	case 0:
		m.timerRow = (m.timerRow + delta + 6) % 6
	case 1:
		m.radioRow = (m.radioRow + delta + 3) % 3
	case 2:
		m.channel = (m.channel + delta + 6) % 6
	}
}

// adjustItem changes only the selected control, preserving independent timer sessions.
func (m *Model) adjustItem(delta int, now time.Time) {
	switch m.page {
	case 0:
		switch m.timerRow {
		case 0:
			m.timer.Switch(timer.Mode((int(m.timer.Mode)+delta+3)%3), now)
		case 1:
			m.timer.SetDuration(m.timer.Mode, m.timer.Durations[m.timer.Mode]+delta)
			m.dirty = true
		case 4:
			m.cfg.Chime = delta > 0
			m.dirty = true
		}
	case 1:
		switch m.radioRow {
		case 0:
			m.changeStation(delta)
		case 1:
			m.sound.RadioVolume = min(100, max(0, m.sound.RadioVolume+delta*5))
			m.dirty = true
		}
	case 2:
		if m.channel < 5 {
			m.sound.Volumes[m.channel] = min(100, max(0, m.sound.Volumes[m.channel]+delta*5))
			m.dirty = true
		}
	}
}

// activateItem performs the selected action; Space remains a global timer shortcut.
func (m *Model) activateItem(now time.Time) {
	switch m.page {
	case 0:
		switch m.timerRow {
		case 0:
			m.adjustItem(1, now)
		case 2:
			m.timer.Toggle(now)
		case 3:
			m.timer.Reset(now)
			m.notice = "Sessão reiniciada."
		case 4:
			m.cfg.Chime = !m.cfg.Chime
			m.dirty = true
		case 5:
			m.sound.Chime++
			m.notice = "Teste do aviso sonoro."
		}
	case 1:
		if m.radioRow == 0 {
			m.changeStation(1)
		}
		if m.radioRow == 2 {
			m.toggleRadio()
		}
	case 2:
		m.toggleMixer()
	}
}

// toggleRadio starts a fresh request or cancels playback, invalidating stale events.
func (m *Model) toggleRadio() {
	m.sound.Radio = !m.sound.Radio
	m.sound.RadioRequest++
	m.radioStatus = "Parado"
	m.notice = "Rádio parado."
	if m.sound.Radio {
		m.radioStatus = string(audio.Connecting)
		m.notice = "Conectando ao rádio…"
	}
}

// toggleMixer changes the requested mixer state and invalidates old status events.
func (m *Model) toggleMixer() {
	m.sound.Mixer = !m.sound.Mixer
	m.sound.MixerRequest++
	m.mixerStatus = "Pausado"
	m.notice = "Sons ambientes pausados."
	if m.sound.Mixer {
		m.mixerStatus = "Preparando"
		m.notice = "Preparando sons ambientes…"
	}
}

// changeStation preserves playback intent while switching the selected station.
func (m *Model) changeStation(delta int) {
	m.sound.Station = (m.sound.Station + delta + len(audio.Stations())) % len(audio.Stations())
	m.sound.RadioRequest++
	if m.sound.Radio {
		m.radioStatus = string(audio.Connecting)
		m.notice = "Conectando ao rádio…"
	} else {
		m.radioStatus = "Parado"
	}
	m.dirty = true
}
