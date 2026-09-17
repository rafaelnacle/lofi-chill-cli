package audio

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPlaybackRequiresPositionAndActiveCore(t *testing.T) {
	cases := []struct {
		name     string
		position any
		idle     any
		failure  string
		want     bool
	}{
		{"playing", 12.5, false, "success", true},
		{"buffering", 12.5, true, "success", false},
		{"not loaded", nil, false, "success", false},
		{"unknown idle", 12.5, nil, "success", false},
		{"unavailable", nil, nil, "property unavailable", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "lofi-ipc-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			socket := filepath.Join(dir, "p.sock")
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(time.Second))
				decoder := json.NewDecoder(conn)
				for range 2 {
					var request map[string]any
					if decoder.Decode(&request) != nil {
						return
					}
				}
				encoder := json.NewEncoder(conn)
				// Unsolicited notifications and out-of-order replies must not confuse status.
				_ = encoder.Encode(map[string]any{"event": "property-change"})
				_ = encoder.Encode(map[string]any{"request_id": 2, "error": tc.failure, "data": tc.idle})
				_ = encoder.Encode(map[string]any{"request_id": 1, "error": tc.failure, "data": tc.position})
			}()
			got := playbackActive(context.Background(), socket)
			if got != tc.want {
				t.Errorf("active=%v; want %v", got, tc.want)
			}
			<-done
		})
	}
}

func TestPlaybackObserverStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	states := make(chan Playback, 1)
	done := make(chan struct{})
	go func() { defer close(done); observePlayback(ctx, "/nonexistent/lofi.sock", states) }()
	select {
	case state := <-states:
		if state != Connecting {
			t.Fatal(state)
		}
	case <-time.After(time.Second):
		t.Fatal("missing connecting state")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("observer did not stop")
	}
}

func TestMPVConfirmsPlaybackWithSilentOutput(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv is not installed")
	}
	dir, err := os.MkdirTemp("", "lofi-mpv-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "p.sock")
	player, err := start(context.Background(), []string{"--no-config", "--no-video", "--no-terminal", "--ao=null", "--loop-file=inf", "--input-ipc-server=" + socket, "--", "assets/rain.mp3"}, false, socket)
	if err != nil {
		t.Fatal(err)
	}
	defer player.stop()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case state := <-player.status:
			if state == Playing {
				return
			}
		case <-timeout:
			t.Fatal("mpv never confirmed playback")
		}
	}
}
