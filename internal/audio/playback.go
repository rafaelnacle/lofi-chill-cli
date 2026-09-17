package audio

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"time"
)

// observePlayback polls JSON IPC outside the PCM worker so a slow player cannot
// interrupt sound generation. Cancellation closes any connection and ends polling.
func observePlayback(ctx context.Context, socket string, states chan Playback) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		state := Connecting
		if playbackActive(ctx, socket) {
			state = Playing
		}
		select {
		case <-states:
		default:
		}
		select {
		case states <- state:
		default:
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// playbackActive reports activity only after mpv confirms both a media position
// and a non-idle core. Missing sockets and buffering are not successful playback.
func playbackActive(ctx context.Context, socket string) bool {
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
	if err != nil {
		return false
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return false
	}
	encoder := json.NewEncoder(conn)
	for id, property := range []string{"time-pos", "core-idle"} {
		if err := encoder.Encode(map[string]any{"command": []any{"get_property", property}, "request_id": id + 1}); err != nil {
			return false
		}
	}
	decoder := json.NewDecoder(io.LimitReader(conn, 64*1024))
	var position, active, seenPosition, seenIdle bool
	for !(seenPosition && seenIdle) {
		var reply struct {
			ID    int             `json:"request_id"`
			Error string          `json:"error"`
			Data  json.RawMessage `json:"data"`
		}
		if err := decoder.Decode(&reply); err != nil {
			return false
		}
		if reply.ID != 1 && reply.ID != 2 {
			continue
		}
		if reply.Error != "success" {
			return false
		}
		if reply.ID == 1 {
			var value *float64
			if json.Unmarshal(reply.Data, &value) != nil {
				return false
			}
			position = value != nil
			seenPosition = true
		}
		if reply.ID == 2 {
			var idle *bool
			if json.Unmarshal(reply.Data, &idle) != nil {
				return false
			}
			active = idle != nil && !*idle
			seenIdle = true
		}
	}
	return position && active
}
