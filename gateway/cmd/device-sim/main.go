package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type outboundFrame struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data"`
	Auth      *authPayload   `json:"auth,omitempty"`
}

type authPayload struct {
	DeviceUID string `json:"device_uid"`
	Token     string `json:"token"`
}

func main() {
	var (
		addr      = flag.String("addr", envOr("SIM_GATEWAY_ADDR", "localhost:9000"), "gateway tcp address")
		deviceUID = flag.String("device", envOr("SIM_DEVICE_UID", "MDVR-0001"), "device uid")
		token     = flag.String("token", envOr("SIM_AUTH_TOKEN", "change-me"), "auth token")
		interval  = flag.Duration("interval", durationOr("SIM_INTERVAL", 5*time.Second), "telemetry interval")
		lat       = flag.Float64("lat", floatOr("SIM_LAT", 40.7128), "starting latitude")
		lng       = flag.Float64("lng", floatOr("SIM_LNG", -74.0060), "starting longitude")
	)
	flag.Parse()

	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		log.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	log.Printf("connected to gateway=%s device=%s", *addr, *deviceUID)

	ctxDone := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(ctxDone) }) }

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Printf("shutdown signal received")
		stop()
	}()

	// First frame must include auth for gateway session registration.
	if err := writeFrame(conn, outboundFrame{
		Type:      "heartbeat",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]any{
			"status": "boot",
		},
		Auth: &authPayload{DeviceUID: *deviceUID, Token: *token},
	}); err != nil {
		log.Fatalf("initial auth frame failed: %v", err)
	}

	go readCommands(conn, ctxDone)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	randSrc := rand.New(rand.NewSource(time.Now().UnixNano()))
	curLat := *lat
	curLng := *lng

	for {
		select {
		case <-ctxDone:
			return
		case t := <-ticker.C:
			if err := writeFrame(conn, outboundFrame{
				Type:      "heartbeat",
				Timestamp: t.UTC().Format(time.RFC3339),
				Data: map[string]any{
					"battery": 86,
				},
			}); err != nil {
				log.Printf("heartbeat send failed: %v", err)
				stop()
				return
			}

			curLat += (randSrc.Float64() - 0.5) * 0.0006
			curLng += (randSrc.Float64() - 0.5) * 0.0006
			speed := 25 + randSrc.Float64()*35
			heading := randSrc.Float64() * 360
			if err := writeFrame(conn, outboundFrame{
				Type:      "location_update",
				Timestamp: t.UTC().Format(time.RFC3339),
				Data: map[string]any{
					"lat":       round(curLat, 6),
					"lng":       round(curLng, 6),
					"speed_kph": round(speed, 2),
					"heading":   round(heading, 2),
				},
			}); err != nil {
				log.Printf("location send failed: %v", err)
				stop()
				return
			}

			if randSrc.Intn(12) == 0 {
				if err := writeFrame(conn, outboundFrame{
					Type:      "alarm",
					Timestamp: t.UTC().Format(time.RFC3339),
					Data: map[string]any{
						"alarm_type": "panic_button",
						"severity":   "high",
					},
				}); err != nil {
					log.Printf("alarm send failed: %v", err)
					stop()
					return
				}
			}
		}
	}
}

func readCommands(conn net.Conn, done <-chan struct{}) {
	reader := bufio.NewReader(conn)
	for {
		select {
		case <-done:
			return
		default:
		}

		head := make([]byte, 2)
		if _, err := io.ReadFull(reader, head); err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				log.Printf("command read closed: %v", err)
			}
			return
		}
		n := int(binary.BigEndian.Uint16(head))
		if n <= 0 || n > 65535 {
			log.Printf("invalid command frame length: %d", n)
			return
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(reader, body); err != nil {
			log.Printf("command read body failed: %v", err)
			return
		}
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			log.Printf("command frame raw=%s", string(body))
			continue
		}
		pretty, _ := json.Marshal(decoded)
		log.Printf("downlink command received=%s", string(pretty))
	}
}

func writeFrame(conn net.Conn, frame outboundFrame) error {
	body, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if len(body) > 65535 {
		return fmt.Errorf("frame too large")
	}
	buf := make([]byte, 2+len(body))
	binary.BigEndian.PutUint16(buf[:2], uint16(len(body)))
	copy(buf[2:], body)
	_, err = conn.Write(buf)
	return err
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func durationOr(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func floatOr(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	var out float64
	if _, err := fmt.Sscanf(v, "%f", &out); err != nil {
		return fallback
	}
	return out
}

func round(v float64, places int) float64 {
	mul := 1.0
	for i := 0; i < places; i++ {
		mul *= 10
	}
	return math.Round(v*mul) / mul
}
