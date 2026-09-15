package ui

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"lofi-chill/internal/audio"
	"lofi-chill/internal/config"
	"lofi-chill/internal/timer"
)

var purple = lipgloss.Color("141")
var pink = lipgloss.Color("212")
var cyan = lipgloss.Color("87")
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var accent = lipgloss.NewStyle().Foreground(cyan).Bold(true)

type tick time.Time
type saved struct{ err error }
type persistence struct {
	mu     sync.Mutex
	closed bool
}
type Model struct {
	writes                       *persistence
	helpOffset                   int
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

func New(c config.Config, path string, engine *audio.Engine, warning error) Model {
	m := Model{writes: &persistence{}, timer: timer.New(time.Now(), c.Durations, c.Day, c.Completed), cfg: c, path: path, engine: engine, width: 80, height: 24, persist: warning == nil, notice: "Uma coisa de cada vez. Seu espaço está pronto."}
	m.sound.Station = c.Station
	m.sound.RadioVolume = c.RadioVolume
	for i, k := range config.Channels {
		m.sound.Volumes[i] = c.Volumes[k]
	}
	if warning != nil {
		m.notice = "Preferências não carregadas; arquivo preservado: " + warning.Error()
	}
	return m
}
func pulse() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tick(t) })
}
func (m Model) listen() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.engine.Events
		if !ok {
			return nil
		}
		return event
	}
}
func (m Model) Init() tea.Cmd { return tea.Batch(pulse(), m.listen()) }
func (m *Model) syncConfig() {
	m.cfg.Durations = m.timer.Durations
	m.cfg.Day = m.timer.Day
	m.cfg.Completed = m.timer.Completed
	m.cfg.Station = m.sound.Station
	m.cfg.RadioVolume = m.sound.RadioVolume
	for i, k := range config.Channels {
		m.cfg.Volumes[k] = m.sound.Volumes[i]
	}
}
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
		m.notice = v.Message
		if v.RadioFailed {
			m.sound.Radio = false
		}
		if v.MixerFailed {
			m.sound.Mixer = false
		}
		m.engine.Set(m.sound)
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
			m.help = false
			return m, nil
		}
		if m.help {
			if key == "down" || key == "j" {
				m.helpOffset = min(20, m.helpOffset+1)
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
		case "shift+tab":
			m.page = (m.page + 2) % 3
		case "f":
			m.focus = !m.focus
		case "m":
			m.sound.Mixer = !m.sound.Mixer
		case "p":
			m.sound.Radio = !m.sound.Radio
		case "[":
			m.sound.Station = (m.sound.Station + 3) % 4
			m.dirty = true
		case "]":
			m.sound.Station = (m.sound.Station + 1) % 4
			m.dirty = true
		case ",":
			m.sound.RadioVolume = max(0, m.sound.RadioVolume-5)
			m.dirty = true
		case ".":
			m.sound.RadioVolume = min(100, m.sound.RadioVolume+5)
			m.dirty = true
		case "up", "k":
			m.channel = (m.channel + 4) % 5
			m.page = 2
		case "down", "j":
			m.channel = (m.channel + 1) % 5
			m.page = 2
		case "left", "h":
			m.sound.Volumes[m.channel] = max(0, m.sound.Volumes[m.channel]-5)
			m.page = 2
			m.dirty = true
		case "right", "l":
			m.sound.Volumes[m.channel] = min(100, m.sound.Volumes[m.channel]+5)
			m.page = 2
			m.dirty = true
		case "c":
			m.cfg.Chime = !m.cfg.Chime
			m.dirty = true
		case "t":
			m.sound.Chime++
			m.notice = "Teste do aviso sonoro."
		}
		m.engine.Set(m.sound)
	}
	if m.timer.Completion != before {
		m.notice = "Sessão concluída. " + timer.Labels[m.timer.Mode] + " pronta para iniciar."
		if m.cfg.Chime {
			m.sound.Chime++
			m.engine.Set(m.sound)
		}
	}
	if day != m.timer.Day || count != m.timer.Completed {
		m.dirty = true
	}
	cmds = append(cmds, m.save())
	return m, tea.Batch(cmds...)
}
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
func on(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}
func (m Model) timerView() string {
	tabs := make([]string, 3)
	for i, s := range [3]string{"Foco", "Curta", "Longa"} {
		tabs[i] = fmt.Sprintf("%d %s", i+1, s)
		if i == int(m.timer.Mode) {
			tabs[i] = accent.Render("[" + tabs[i] + "]")
		}
	}
	session := m.timer.Sessions[m.timer.Mode]
	sec := int(math.Ceil(session.Remaining.Seconds()))
	status := "Iniciar"
	if !m.timer.Deadline.IsZero() {
		status = "Pausar · em andamento"
	} else if session.Started {
		status = "Retomar / Resume · pausado"
	}
	clock := fmt.Sprintf("%02d:%02d", sec/60, sec%60)
	if m.width >= 46 && m.height >= 22 {
		clock = bigClock(clock)
	}
	return strings.Join(tabs, "  ") + "\n\n" + lipgloss.NewStyle().Foreground(pink).Bold(true).Render("  "+clock) + "\n\n" + accent.Render("[espaço] "+status) + fmt.Sprintf("\n[r] Reset   [-/+] Duração: %d min\n\n%d focos hoje   •   Aviso %s [c]  Testar [t]", m.timer.Durations[m.timer.Mode], m.timer.Completed, on(m.cfg.Chime))
}
func (m Model) radioView() string {
	return accent.Render("RÁDIO LOFI") + "\n\n" + audio.Stations[m.sound.Station] + fmt.Sprintf("\n\n[%s]  [p] Tocar/parar\n[,] Volume %d%% [.]\n[ / ] Estação anterior/próxima", on(m.sound.Radio), m.sound.RadioVolume)
}
func (m Model) mixerView() string {
	rows := []string{accent.Render("SET THE MOOD") + "  [m] " + on(m.sound.Mixer), ""}
	for i, name := range audio.Names {
		v := m.sound.Volumes[i]
		bar := strings.Repeat("━", v/10) + strings.Repeat("·", 10-v/10)
		prefix := "  "
		if i == m.channel {
			prefix = "> "
		}
		row := fmt.Sprintf("%s%-12s %s %3d%%", prefix, name, bar, v)
		if i == m.channel {
			row = accent.Render(row)
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n") + "\n\n↑/↓ Canal   ←/→ Volume"
}
func panel(content string, width int) string {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(purple).Width(width - 4).Padding(1).Render(content)
}
func (m Model) View() string {
	if m.width < 30 || m.height < 10 {
		return fit("lofi & chill\nAmplie para 30×10.\nEspaço: timer · q: sair", m.width, m.height)
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(pink).Render("▰ lofi & chill") + muted.Render("  /  SIDE A — FOCUS")
	footer := muted.Render("espaço timer · tab painel · f foco · ? ajuda · q sair")
	var body string
	if m.help {
		body = "ATALHOS\n\n1 / 2 / 3   Foco / pausa curta / longa\nespaço      Iniciar, pausar ou retomar\nr           Reset da sessão atual\n- / +       Duração do modo (1–120 min)\np           Tocar/parar rádio\n[ / ]       Escolher estação\n, / .       Volume do rádio\nm           Play/pause do mixer\n↑ / ↓       Selecionar canal (j/k)\n← / →       Volume do canal (h/l)\nc / t       Aviso on/off / testar\ntab         Trocar painel\nf           Modo foco\n? / esc     Fechar ajuda\nq / ctrl+c  Sair\n\nÁudio e sessões não retomam ao reabrir."
	} else if m.focus {
		body = panel(m.timerView(), m.width)
	} else if m.width >= 100 && m.height >= 31 {
		left := m.width / 2
		body = lipgloss.JoinHorizontal(lipgloss.Top, panel(m.timerView(), left), panel(m.radioView(), m.width-left)) + "\n" + panel(m.mixerView(), m.width)
	} else {
		nav := []string{"POMODORO", "RÁDIO", "MIXER"}
		for i := range nav {
			if i == m.page {
				nav[i] = accent.Render("[" + nav[i] + "]")
			}
		}
		body = strings.Join(nav, "  ") + "\n"
		views := []string{m.timerView(), m.radioView(), m.mixerView()}
		body += views[m.page]
	}
	if m.height < 18 && !m.help {
		session := m.timer.Sessions[m.timer.Mode]
		sec := int(math.Ceil(session.Remaining.Seconds()))
		if m.page == 0 || m.focus {
			body = fmt.Sprintf("%s  %02d:%02d\nEspaço: iniciar/pausar/retomar\n1/2/3 modo · r reset · +/- duração\n%d min · %d focos hoje · aviso %s", timer.Labels[m.timer.Mode], sec/60, sec%60, m.timer.Durations[m.timer.Mode], m.timer.Completed, on(m.cfg.Chime))
		} else if m.page == 2 {
			body = fmt.Sprintf("MIXER %s [m] · canal %d/5\n%s: %d%%\n↑/↓ canal · ←/→ volume", on(m.sound.Mixer), m.channel+1, audio.Names[m.channel], m.sound.Volumes[m.channel])
		} else {
			body = fmt.Sprintf("RÁDIO %s [p]\n%s\n[/] estação · ,/. volume %d%%", on(m.sound.Radio), audio.Stations[m.sound.Station], m.sound.RadioVolume)
		}
	}
	lines := strings.Split(body, "\n")
	if m.help {
		m.helpOffset = min(m.helpOffset, max(0, len(lines)-max(1, m.height-5)))
		lines = lines[m.helpOffset:]
		footer = muted.Render("↑/↓ rolar · ?/esc voltar · q sair")
	}
	budget := m.height - 5
	if len(lines) > budget {
		lines = lines[:max(0, budget)]
	}
	return fit(title+"\n\n"+strings.Join(lines, "\n")+"\n"+muted.Render(m.notice)+"\n"+footer, m.width, m.height)
}
func fit(s string, width, height int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:max(0, height)]
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], max(0, width), "…")
	}
	return strings.Join(lines, "\n")
}

func bigClock(value string) string {
	glyphs := map[rune][3]string{
		'0': {"█▀█", "█ █", "█▄█"}, '1': {" ▀█", "  █", "  █"}, '2': {"▀▀█", "█▀▀", "█▄▄"}, '3': {"▀▀█", " ▀█", "▄▄█"}, '4': {"█ █", "▀▀█", "  █"}, '5': {"█▀▀", "▀▀█", "▄▄█"}, '6': {"█▀▀", "█▀█", "█▄█"}, '7': {"▀▀█", "  █", "  █"}, '8': {"█▀█", "█▀█", "█▄█"}, '9': {"█▀█", "▀▀█", "▄▄█"}, ':': {" ▄ ", "   ", " ▀ "},
	}
	rows := [3]string{}
	for _, digit := range value {
		for row := range rows {
			rows[row] += glyphs[digit][row] + " "
		}
	}
	return strings.Join(rows[:], "\n  ")
}
