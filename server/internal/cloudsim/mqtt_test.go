package cloudsim

import "testing"

func TestPublishMeta(t *testing.T) {
	topic := "v1_1/W1106000/B0E9FEFB3AEC/propertyChanged"
	body := []byte{byte(len(topic) >> 8), byte(len(topic))}
	body = append(body, topic...)
	body = append(body, 0x12, 0x34) // packet id, present only at QoS > 0
	body = append(body, `{"code":1}`...)

	p := mqttPacket{typ: pktPublish, flags: 0x02, body: body} // QoS 1
	gotTopic, gotID, gotQoS := p.publishMeta()
	if gotTopic != topic {
		t.Errorf("topic = %q, want %q", gotTopic, topic)
	}
	if gotID != 0x1234 {
		t.Errorf("id = %#x, want 0x1234", gotID)
	}
	if gotQoS != 1 {
		t.Errorf("qos = %d, want 1", gotQoS)
	}
}

// A QoS 0 publish carries no packet identifier; reading one anyway would take
// two bytes of payload and produce a bogus id to acknowledge.
func TestPublishMetaQoS0HasNoPacketID(t *testing.T) {
	topic := "t"
	body := append([]byte{0x00, 0x01}, topic...)
	body = append(body, "payload"...)

	p := mqttPacket{typ: pktPublish, flags: 0x00, body: body}
	_, id, qos := p.publishMeta()
	if qos != 0 {
		t.Fatalf("qos = %d, want 0", qos)
	}
	if id != 0 {
		t.Errorf("id = %d, want 0 for a QoS 0 publish", id)
	}
}

func TestPublishMetaTruncated(t *testing.T) {
	// Claims a 300-byte topic but carries none: must not panic or over-read.
	p := mqttPacket{typ: pktPublish, flags: 0x02, body: []byte{0x01, 0x2c}}
	if topic, _, _ := p.publishMeta(); topic != "" {
		t.Errorf("topic = %q, want empty", topic)
	}
}

func TestSubscribeMetaCountsFilters(t *testing.T) {
	body := []byte{0x00, 0x07}
	for _, f := range []string{"a/b", "c/d/e"} {
		body = append(body, byte(len(f)>>8), byte(len(f)))
		body = append(body, f...)
		body = append(body, 0x00) // requested QoS
	}
	p := mqttPacket{typ: pktSubscribe, body: body}
	id, topics := p.subscribeMeta()
	if id != 7 {
		t.Errorf("id = %d, want 7", id)
	}
	if len(topics) != 2 || topics[0] != "a/b" || topics[1] != "c/d/e" {
		t.Errorf("topics = %q, want [a/b c/d/e]", topics)
	}
}

// UNSUBSCRIBE has no per-filter QoS byte, so it needs a different stride.
func TestSubscribeMetaUnsubscribeStride(t *testing.T) {
	body := []byte{0x00, 0x09}
	for _, f := range []string{"a/b", "c/d/e"} {
		body = append(body, byte(len(f)>>8), byte(len(f)))
		body = append(body, f...)
	}
	p := mqttPacket{typ: pktUnsubscribe, body: body}
	id, topics := p.subscribeMeta()
	if id != 9 {
		t.Errorf("id = %d, want 9", id)
	}
	if len(topics) != 2 || topics[0] != "a/b" || topics[1] != "c/d/e" {
		t.Errorf("topics = %q, want [a/b c/d/e]", topics)
	}
}
