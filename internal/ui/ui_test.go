package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"lofi-chill/internal/audio"
	"lofi-chill/internal/config"
	"lofi-chill/internal/timer"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNavigationAndResume(t *testing.T) {
	e := audio.New()
	defer e.Close()
	m := New(config.Default(), filepath.Join(t.TempDir(), "config.json"), e, nil)
	now := time.Now()
	m.timer.Toggle(now)
	m.timer.Switch(timer.Short, now.Add(time.Minute))
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = next.(Model)
	if !strings.Contains(m.View(), "Retomar") {
		t.Fatal(m.View())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if !strings.Contains(m.View(), "Lofi Girl") {
		t.Fatal(m.View())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.channel != 0 || m.page != 1 || m.radioRow != 0 {
		t.Fatal(m.channel, m.page)
	}
}
func TestLayoutFitsAndHelpScrolls(t *testing.T) {
	m := New(config.Default(), "", nil, nil)
	for _, size := range [][2]int{{20, 8}, {30, 10}, {60, 18}, {80, 24}, {120, 40}} {
		m.width, m.height = size[0], size[1]
		for page := 0; page < 3; page++ {
			m.page = page
			view := m.View()
			lines := strings.Split(view, "\n")
			if len(lines) > m.height {
				t.Fatal(size, len(lines))
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > m.width {
					t.Fatal(size, line)
				}
			}
		}
	}
	m.width = 60
	m.height = 18
	m.help = true
	for i := 0; i < 20; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
	}
	if !strings.Contains(m.View(), "não retomam") {
		t.Fatal(m.View())
	}
}
func TestExitCannotBeOverwrittenByPendingSave(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	m := New(config.Default(), p, nil, nil)
	m.dirty = true
	pending := m.save()
	m.sound.RadioVolume = 75
	if err := m.SaveOnExit(); err != nil {
		t.Fatal(err)
	}
	pending()
	c, e := config.Load(p)
	if e != nil || c.RadioVolume != 75 {
		t.Fatal(c, e)
	}
}

func TestContextualControlsAndGlobalTimer(t *testing.T) {
	m := New(config.Default(), "", nil, nil)
	m.persist = false
	// Radio volume changes must not move the mixer selection or touch its levels.
	m.page = 1
	m.radioRow = 1
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.page != 1 || m.sound.RadioVolume != 55 || m.sound.Volumes[0] != 35 {
		t.Fatal(m.sound)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(Model)
	if m.timer.Deadline.IsZero() {
		t.Fatal("Space did not start timer from radio")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.page != 2 || m.sound.Volumes[0] != 40 {
		t.Fatal(m.sound)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.sound.Mixer || m.mixerStatus != "Preparando" {
		t.Fatal("Enter did not start selected mixer")
	}
}

func TestStalePlaybackEventsCannotOverrideNewRequests(t *testing.T) {
	m := New(config.Default(), "", nil, nil)
	m.persist = false
	m.toggleRadio()
	old := m.sound.RadioRequest
	m.changeStation(1)
	next, _ := m.Update(audio.Event{RadioFailed: true, RadioRequest: old, Message: "old failure"})
	m = next.(Model)
	if !m.sound.Radio || m.radioStatus != "Conectando" {
		t.Fatal("stale failure stopped new station")
	}
	next, _ = m.Update(audio.Event{RadioStatus: audio.Playing, RadioRequest: m.sound.RadioRequest})
	m = next.(Model)
	if m.radioStatus != "Tocando" {
		t.Fatal(m.radioStatus)
	}
	m.toggleRadio()
	next, _ = m.Update(audio.Event{RadioStatus: audio.Playing, RadioRequest: m.sound.RadioRequest - 1})
	m = next.(Model)
	if m.radioStatus != "Parado" {
		t.Fatal("stale playback resurrected stopped radio")
	}
}

func TestFixedFooterAndPersistentTimer(t *testing.T) {
	for _, size := range [][2]int{{30, 10}, {40, 12}, {80, 24}, {96, 26}, {100, 31}, {120, 40}} {
		for page := 0; page < 3; page++ {
			m := New(config.Default(), "", nil, nil)
			m.width, m.height = size[0], size[1]
			m.page = page
			m.timer.Toggle(time.Now())
			view := ansi.Strip(m.View())
			lines := strings.Split(view, "\n")
			if len(lines) != m.height {
				t.Fatalf("%v: got %d rows", size, len(lines))
			}
			if !strings.Contains(lines[1], "25:00") || !strings.Contains(lines[1], "Em andamento") {
				t.Fatalf("missing timer at %v: %s", size, lines[1])
			}
			if !strings.Contains(lines[len(lines)-1], "q") || !strings.Contains(lines[len(lines)-1], "Tab") {
				t.Fatalf("missing footer: %q", lines[len(lines)-1])
			}
			if size[0] >= 96 && size[1] >= 26 {
				if !strings.Contains(view, "Lareira") || strings.Count(view, "╰") != 3 || strings.Count(view, "╯") != 3 {
					t.Fatalf("clipped wide layout at %v:\n%s", size, view)
				}
			}
		}
	}
}

func TestSmallLayoutKeepsSelectedActionVisible(t *testing.T) {
	m := New(config.Default(), "", nil, nil)
	m.width = 30
	m.height = 10
	for page := 0; page < 3; page++ {
		m.page = page
		rows, _ := m.items(page)
		for index := range rows {
			switch page {
			case 0:
				m.timerRow = index
			case 1:
				m.radioRow = index
			case 2:
				m.channel = index
			}
			lines := strings.Split(ansi.Strip(m.View()), "\n")
			if !strings.Contains(strings.Join(lines[3:m.height-4], "\n"), "> ") {
				t.Fatalf("selection hidden on panel %d item %d", page, index)
			}
		}
	}
}

func TestWideFocusAndEscape(t *testing.T) {
	m := New(config.Default(), "", nil, nil)
	m.width = 120
	m.height = 40
	first := m.View()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.View() == first || !strings.Contains(ansi.Strip(m.View()), "> RÁDIO · selecionado") {
		t.Fatal("Tab has no visible focus")
	}
	m.focus = true
	m.page = 0
	m.help = true
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.help || !m.focus {
		t.Fatal("Escape should close only the help overlay")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.focus {
		t.Fatal("Escape should exit focus mode")
	}
}

func TestPanelsHonorTheirSizeBudget(t *testing.T) {
	m := New(config.Default(), "", nil, nil)
	for _, size := range [][2]int{{40, 10}, {49, 24}, {80, 17}} {
		for page := 0; page < 3; page++ {
			lines := strings.Split(m.box(page, size[0], size[1]), "\n")
			if len(lines) != size[1] {
				t.Fatalf("panel %d: %d rows, want %d", page, len(lines), size[1])
			}
			for _, line := range lines {
				if ansi.StringWidth(line) != size[0] {
					t.Fatalf("panel %d: width %d, want %d", page, ansi.StringWidth(line), size[0])
				}
			}
		}
	}
}

func TestSpaceRemainsAvailableInHelp(t *testing.T) {
	m := New(config.Default(), "", nil, nil)
	m.help = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(Model)
	if !m.help || m.timer.Deadline.IsZero() {
		t.Fatal("Space must control the timer without closing help")
	}
}
