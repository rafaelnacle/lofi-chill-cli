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
	if !strings.Contains(m.View(), "Resume") {
		t.Fatal(m.View())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if !strings.Contains(m.View(), "Lofi Girl") {
		t.Fatal(m.View())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.channel != 1 || m.page != 2 {
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
