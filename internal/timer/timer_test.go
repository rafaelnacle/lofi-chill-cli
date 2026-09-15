package timer

import (
	"testing"
	"time"
)

func base() (Timer, time.Time) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.Local)
	return New(now, [3]int{25, 5, 15}, "", 0), now
}
func TestSwitchAndDurationPreserve(t *testing.T) {
	s, n := base()
	s.Toggle(n)
	s.Switch(Short, n.Add(70*time.Second))
	s.SetDuration(Focus, 40)
	s.Switch(Focus, n.Add(5*time.Minute))
	if s.Sessions[Focus].Remaining != 25*time.Minute-70*time.Second || !s.Sessions[Focus].Started || !s.Deadline.IsZero() {
		t.Fatal(s)
	}
	s.Toggle(n.Add(6 * time.Minute))
	if s.Deadline.Sub(n.Add(6*time.Minute)) != 25*time.Minute-70*time.Second {
		t.Fatal(s)
	}
	s.Reset(n)
	if s.Sessions[Focus].Remaining != 40*time.Minute || s.Completed != 0 {
		t.Fatal(s)
	}
}
func TestCompletionOnceAndManualBreak(t *testing.T) {
	s, n := base()
	s.Toggle(n)
	if !s.Tick(n.Add(30 * time.Minute)) {
		t.Fatal("not completed")
	}
	s.Tick(n.Add(time.Hour))
	if s.Completed != 1 || s.Completion != 1 || s.Mode != Short || !s.Deadline.IsZero() {
		t.Fatal(s)
	}
}
func TestSwitchAtDeadline(t *testing.T) {
	for _, mode := range []Mode{Focus, Short, Long} {
		s, n := base()
		s.Toggle(n)
		s.Switch(mode, n.Add(25*time.Minute))
		s.Tick(n.Add(time.Hour))
		if s.Completed != 1 || s.Completion != 1 || s.Mode != mode || !s.Deadline.IsZero() {
			t.Fatal(mode, s)
		}
	}
}
func TestBreakReturnsToPausedFocus(t *testing.T) {
	s, n := base()
	s.Toggle(n)
	s.Switch(Short, n.Add(time.Minute))
	s.Toggle(n.Add(time.Minute))
	s.Tick(n.Add(6 * time.Minute))
	if s.Mode != Focus || s.Sessions[Focus].Remaining != 24*time.Minute || !s.Sessions[Focus].Started || s.Completed != 0 {
		t.Fatal(s)
	}
}
func TestFourthFocusLongBreak(t *testing.T) {
	s, n := base()
	for i := 0; i < 4; i++ {
		s.Switch(Focus, n)
		s.Toggle(n)
		n = n.Add(25 * time.Minute)
		s.Tick(n)
	}
	if s.Mode != Long || s.Completed != 4 {
		t.Fatal(s)
	}
}
func TestDayRolloverAndResetCancellation(t *testing.T) {
	s, n := base()
	s.Completed = 3
	s.Toggle(n)
	s.Reset(n.Add(25 * time.Minute))
	if s.Completed != 3 || s.Completion != 0 {
		t.Fatal(s)
	}
	s.Tick(n.AddDate(0, 0, 1))
	if s.Completed != 0 {
		t.Fatal(s)
	}
}
func TestRestoreStartsFreshAndValidates(t *testing.T) {
	s, n := base()
	s = New(n, [3]int{0, 121, 12}, n.Format("2006-01-02"), 7)
	if s.Completed != 7 || s.Durations != [3]int{25, 5, 12} || s.Sessions[0].Started {
		t.Fatal(s)
	}
	s.SetDuration(Focus, 120)
	s.SetDuration(Focus, 121)
	if s.Sessions[0].Remaining != 120*time.Minute {
		t.Fatal(s)
	}
}
