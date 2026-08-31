package cloudsim

import (
	"encoding/json"
	"net"
	"testing"
)

func TestSetPropertyWithoutSessionFails(t *testing.T) {
	s := newTestServer(t)
	// Silently succeeding here is the old bug: the setting would look applied
	// in the UI while never reaching the robot.
	if err := s.SetProperty(1039, 55); err == nil {
		t.Fatal("expected an error when no session is connected")
	}
	if _, ok := s.Property(1039); ok {
		t.Error("value should not be stored when it could not be delivered")
	}
}

func TestSetPropertyPublishesAndPersists(t *testing.T) {
	s := newTestServer(t)
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	topic := "v1_1/W1106000/B0E9FEFB3AEC/propertySet"
	s.adoptSession(server, []string{"v1_1/W1106000/B0E9FEFB3AEC/funcRequest", topic})

	got := make(chan mqttPacket, 1)
	go func() {
		p, err := readPacket(client)
		if err == nil {
			got <- p
		}
	}()

	if err := s.SetProperty(1039, 55); err != nil {
		t.Fatalf("SetProperty: %v", err)
	}

	pkt := <-got
	if pkt.typ != pktPublish {
		t.Fatalf("packet type = %d, want PUBLISH", pkt.typ)
	}
	gotTopic, _, _ := pkt.publishMeta()
	if gotTopic != topic {
		t.Errorf("topic = %q, want %q", gotTopic, topic)
	}

	// The envelope has to match what control_center's onCloudPropertySet
	// expects: a payload object carrying a property object.
	body := pkt.body[2+len(gotTopic):]
	var env struct {
		DeviceID string `json:"deviceID"`
		Version  string `json:"version"`
		Payload  struct {
			Property map[string]any `json:"property"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("payload is not valid json: %v (%s)", err, body)
	}
	if env.DeviceID != "B0E9FEFB3AEC" {
		t.Errorf("deviceID = %q, want B0E9FEFB3AEC", env.DeviceID)
	}
	if env.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", env.Version)
	}
	if v, ok := env.Payload.Property["1039"]; !ok || v.(float64) != 55 {
		t.Errorf("property = %v, want {\"1039\": 55}", env.Payload.Property)
	}

	// Persisted, so a control_center restart can be replayed to.
	if v, ok := s.Property(1039); !ok || v != any(55) {
		t.Errorf("stored value = %v (ok=%v), want 55", v, ok)
	}
	reopened, err := loadProperties(s.cfg.CertDir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if v, ok := reopened.get(1039); !ok || v.(float64) != 55 {
		t.Errorf("value did not survive a reload: %v (ok=%v)", v, ok)
	}
}

func TestDeviceIDFromTopic(t *testing.T) {
	tests := map[string]string{
		"v1_1/W1106000/B0E9FEFB3AEC/propertySet": "B0E9FEFB3AEC",
		"short":                                  "",
	}
	for topic, want := range tests {
		if got := deviceIDFromTopic(topic); got != want {
			t.Errorf("deviceIDFromTopic(%q) = %q, want %q", topic, got, want)
		}
	}
}

func TestEncodePublishRoundTrips(t *testing.T) {
	// A payload over 127 bytes forces a multi-byte remaining-length, which is
	// where a hand-rolled encoder usually goes wrong.
	payload := make([]byte, 300)
	for i := range payload {
		payload[i] = 'x'
	}
	frame := encodePublish("a/topic", payload)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	go func() { server.Write(frame) }()

	pkt, err := readPacket(client)
	if err != nil {
		t.Fatalf("readPacket: %v", err)
	}
	topic, _, _ := pkt.publishMeta()
	if topic != "a/topic" {
		t.Errorf("topic = %q, want a/topic", topic)
	}
	if body := pkt.body[2+len(topic):]; len(body) != len(payload) {
		t.Errorf("payload length = %d, want %d", len(body), len(payload))
	}
}

// A value read back in the same process is still an int or bool; only after a
// reload has it been through JSON and become a float64. Both must decode, or a
// setting shows its default until mundus restarts.
func TestSettingsBackendReadsBackWrittenValues(t *testing.T) {
	s := newTestServer(t)
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	go func() {
		for {
			if _, err := readPacket(client); err != nil {
				return
			}
		}
	}()
	s.adoptSession(server, []string{"v1_1/W1106000/B0E9FEFB3AEC/propertySet"})

	b := s.Settings()
	if err := b.Set(1039, 0); err != nil { // volume 0, whose default is 50
		t.Fatalf("Set: %v", err)
	}
	if err := b.Set(1062, false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := b.GetInt(1039, 50); got != 0 {
		t.Errorf("GetInt = %d, want the written 0 rather than the default", got)
	}
	if got := b.GetBool(1062, true); got {
		t.Error("GetBool = true, want the written false")
	}

	// And again once the values have round-tripped through JSON.
	reloaded, err := loadProperties(s.cfg.CertDir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	s.props = reloaded
	if got := b.GetInt(1039, 50); got != 0 {
		t.Errorf("after reload GetInt = %d, want 0", got)
	}
	if got := b.GetBool(1062, true); got {
		t.Error("after reload GetBool = true, want false")
	}
}
