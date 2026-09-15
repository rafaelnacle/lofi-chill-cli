// Package timer implements Pomodoro sessions without starting clocks or goroutines.
package timer

import "time"

// Mode identifies a focus session, short break, or long break.
type Mode int

// The Pomodoro modes also index session and duration arrays.
const (
	Focus Mode = iota // Focus is a work session.
	Short             // Short is a short break.
	Long              // Long is a long break.
)

// String returns the Portuguese label for m, or "Desconhecido" for an invalid mode.
func (m Mode) String() string {
	switch m {
	case Focus:
		return "Foco"
	case Short:
		return "Pausa curta"
	case Long:
		return "Pausa longa"
	default:
		return "Desconhecido"
	}
}

// Session holds progress for one mode, including whether Resume should be offered.
type Session struct {
	Remaining time.Duration // Remaining is the paused or last observed countdown.
	Started   bool          // Started distinguishes a paused session from a new one.
}

// Timer preserves three independent sessions and runs at most one countdown.
// Its owner must serialize method calls and use a consistent local clock.
type Timer struct {
	Mode       Mode       // Mode selects the session shown and controlled.
	Sessions   [3]Session // Sessions preserves progress independently for each mode.
	Durations  [3]int     // Durations contains configured minutes, indexed by mode.
	Deadline   time.Time  // Deadline is zero when paused; otherwise it determines completion.
	Day        string     // Day is the local counting date in YYYY-MM-DD format.
	Completed  int        // Completed counts focus sessions finished on Day.
	Completion int        // Completion counts all completion events during this run.
}

// New returns a stopped timer with fresh sessions.
// Invalid durations use 25/5/15-minute defaults; the count is restored only
// when day matches now's local date and completed is nonnegative.
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

// reset replaces one session with its configured duration and clears its progress.
func (t *Timer) reset(m Mode) {
	t.Sessions[m] = Session{Remaining: time.Duration(t.Durations[m]) * time.Minute}
}

// Tick updates elapsed time and resets the daily count when the local date changes.
// It returns true exactly when a running session completes, preparing the next
// mode without starting it or discarding that mode's paused progress.
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
	if old != Focus {
		t.Mode = Focus
		return true
	}
	t.Completed++
	t.Mode = Short
	if t.Completed%4 == 0 {
		t.Mode = Long
	}
	return true
}

// Toggle starts, resumes, or pauses the current session at now.
// If it observes a completion, it leaves the next session stopped.
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

// Switch accounts for elapsed time, then selects m and preserves paused progress.
// Selecting the current mode leaves it running. Invalid modes are ignored.
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

// Reset cancels the selected session and applies its configured duration.
// It updates the counting date but never records a completion, even at the deadline.
func (t *Timer) Reset(now time.Time) {
	if day := now.Format("2006-01-02"); day != t.Day {
		t.Day = day
		t.Completed = 0
	}
	t.Deadline = time.Time{}
	t.reset(t.Mode)
}

// SetDuration changes the configured minutes for m without changing started sessions.
// Modes outside Focus through Long and durations outside 1–120 minutes are ignored.
func (t *Timer) SetDuration(m Mode, minutes int) {
	if m < Focus || m > Long || minutes < 1 || minutes > 120 {
		return
	}
	t.Durations[m] = minutes
	if !t.Sessions[m].Started {
		t.reset(m)
	}
}
