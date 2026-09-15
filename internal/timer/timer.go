package timer

import "time"

type Mode int

const (
	Focus Mode = iota
	Short
	Long
)

var Labels = [3]string{"Foco", "Pausa curta", "Pausa longa"}

type Session struct {
	Remaining time.Duration
	Started   bool
}
type Timer struct {
	Mode       Mode
	Sessions   [3]Session
	Durations  [3]int
	Deadline   time.Time
	Day        string
	Completed  int
	Completion int
}

func New(now time.Time, durations [3]int, day string, completed int) Timer {
	t := Timer{Durations: durations, Day: now.Format("2006-01-02")}
	if day == t.Day && completed >= 0 {
		t.Completed = completed
	}
	for i := range t.Sessions {
		if t.Durations[i] < 1 || t.Durations[i] > 120 {
			t.Durations[i] = [3]int{25, 5, 15}[i]
		}
		t.reset(Mode(i))
	}
	return t
}
func (t *Timer) reset(m Mode) {
	t.Sessions[m] = Session{Remaining: time.Duration(t.Durations[m]) * time.Minute}
}
func (t *Timer) Tick(now time.Time) bool {
	if day := now.Format("2006-01-02"); day != t.Day {
		t.Day = day
		t.Completed = 0
	}
	if t.Deadline.IsZero() {
		return false
	}
	left := t.Deadline.Sub(now)
	if left > 0 {
		t.Sessions[t.Mode].Remaining = left
		return false
	}
	old := t.Mode
	t.reset(old)
	t.Deadline = time.Time{}
	t.Completion++
	if old == Focus {
		t.Completed++
		t.Mode = Short
		if t.Completed%4 == 0 {
			t.Mode = Long
		}
	} else {
		t.Mode = Focus
	}
	return true
}
func (t *Timer) Toggle(now time.Time) {
	if t.Tick(now) {
		return
	}
	if t.Deadline.IsZero() {
		t.Deadline = now.Add(t.Sessions[t.Mode].Remaining)
		t.Sessions[t.Mode].Started = true
	} else {
		t.Deadline = time.Time{}
	}
}
func (t *Timer) Switch(m Mode, now time.Time) {
	if m < Focus || m > Long {
		return
	}
	t.Tick(now)
	if m == t.Mode {
		return
	}
	t.Deadline = time.Time{}
	t.Mode = m
}
func (t *Timer) Reset(now time.Time) {
	// Reset cancels the selected session; it never records a completion.
	if day := now.Format("2006-01-02"); day != t.Day {
		t.Day = day
		t.Completed = 0
	}
	t.Deadline = time.Time{}
	t.reset(t.Mode)
}
func (t *Timer) SetDuration(m Mode, minutes int) {
	if m < Focus || m > Long || minutes < 1 || minutes > 120 {
		return
	}
	t.Durations[m] = minutes
	if !t.Sessions[m].Started {
		t.reset(m)
	}
}
