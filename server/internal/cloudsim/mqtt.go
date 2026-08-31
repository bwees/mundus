package cloudsim

import (
	"fmt"
	"io"
	"net"
)

// MQTT control packet types used by control_center's client.
const (
	pktConnect     = 1
	pktConnack     = 2
	pktPublish     = 3
	pktPuback      = 4
	pktSubscribe   = 8
	pktSuback      = 9
	pktUnsubscribe = 10
	pktUnsuback    = 11
	pktPingreq     = 12
	pktPingresp    = 13
	pktDisconnect  = 14
)

func (s *Server) acceptMQTT(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go s.handleMQTT(c)
	}
}

// handleMQTT speaks just enough MQTT 3.1.1 to keep control_center believing it
// has a cloud session: acknowledge the handshake and subscriptions, ack every
// publish, answer keepalives. Published telemetry is discarded -- accepting it
// is the whole point, since nothing may leave the device.
func (s *Server) handleMQTT(c net.Conn) {
	defer c.Close()
	defer s.clearSession(c)
	for {
		pkt, err := readPacket(c)
		if err != nil {
			if err != io.EOF {
				s.cfg.Log.Debug("cloudsim mqtt read", "err", err)
			}
			return
		}
		switch pkt.typ {
		case pktConnect:
			s.cfg.Log.Info("cloudsim mqtt session opened")
			c.Write([]byte{pktConnack << 4, 0x02, 0x00, 0x00})
		case pktPublish:
			topic, id, qos := pkt.publishMeta()
			s.cfg.Log.Debug("cloudsim mqtt publish discarded", "topic", topic, "qos", qos)
			if qos == 1 {
				c.Write([]byte{pktPuback << 4, 0x02, byte(id >> 8), byte(id)})
			}
		case pktSubscribe:
			id, topics := pkt.subscribeMeta()
			ack := []byte{pktSuback << 4, byte(2 + len(topics)), byte(id >> 8), byte(id)}
			for range topics {
				ack = append(ack, 0x00) // granted QoS 0
			}
			c.Write(ack)
			s.adoptSession(c, topics)
		case pktUnsubscribe:
			id, _ := pkt.subscribeMeta()
			c.Write([]byte{pktUnsuback << 4, 0x02, byte(id >> 8), byte(id)})
		case pktPingreq:
			c.Write([]byte{pktPingresp << 4, 0x00})
		case pktDisconnect:
			s.cfg.Log.Info("cloudsim mqtt session closed")
			return
		}
	}
}

type mqttPacket struct {
	typ   byte
	flags byte
	body  []byte
}

func readPacket(c net.Conn) (mqttPacket, error) {
	hdr := make([]byte, 1)
	if _, err := io.ReadFull(c, hdr); err != nil {
		return mqttPacket{}, err
	}
	n, err := readVarint(c)
	if err != nil {
		return mqttPacket{}, err
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c, body); err != nil {
		return mqttPacket{}, err
	}
	return mqttPacket{typ: hdr[0] >> 4, flags: hdr[0] & 0x0f, body: body}, nil
}

func readVarint(c net.Conn) (int, error) {
	var value, mult int
	b := make([]byte, 1)
	for i := 0; i < 4; i++ {
		if _, err := io.ReadFull(c, b); err != nil {
			return 0, err
		}
		value |= int(b[0]&0x7f) << mult
		if b[0]&0x80 == 0 {
			return value, nil
		}
		mult += 7
	}
	return 0, fmt.Errorf("malformed remaining length")
}

func (p mqttPacket) publishMeta() (topic string, id int, qos byte) {
	qos = (p.flags >> 1) & 0x03
	if len(p.body) < 2 {
		return "", 0, qos
	}
	tl := int(p.body[0])<<8 | int(p.body[1])
	if len(p.body) < 2+tl {
		return "", 0, qos
	}
	topic = string(p.body[2 : 2+tl])
	if qos > 0 && len(p.body) >= 2+tl+2 {
		id = int(p.body[2+tl])<<8 | int(p.body[2+tl+1])
	}
	return topic, id, qos
}

// subscribeMeta returns the packet id and the topic filters. The filters name
// the device and model (v1_1/<model>/<deviceID>/<channel>), which is how the
// publish topic is learned rather than hardcoded.
func (p mqttPacket) subscribeMeta() (id int, topics []string) {
	if len(p.body) < 2 {
		return 0, nil
	}
	id = int(p.body[0])<<8 | int(p.body[1])
	rest := p.body[2:]
	for len(rest) >= 2 {
		tl := int(rest[0])<<8 | int(rest[1])
		// SUBSCRIBE carries a QoS byte per filter; UNSUBSCRIBE does not.
		step := 2 + tl
		if p.typ == pktSubscribe {
			step++
		}
		if len(rest) < step {
			break
		}
		topics = append(topics, string(rest[2:2+tl]))
		rest = rest[step:]
	}
	return id, topics
}

// encodePublish builds a QoS 0 PUBLISH frame.
func encodePublish(topic string, payload []byte) []byte {
	var body []byte
	body = append(body, byte(len(topic)>>8), byte(len(topic)))
	body = append(body, topic...)
	body = append(body, payload...)

	frame := []byte{pktPublish << 4}
	n := len(body)
	for {
		b := byte(n % 128)
		n /= 128
		if n > 0 {
			b |= 0x80
		}
		frame = append(frame, b)
		if n == 0 {
			break
		}
	}
	return append(frame, body...)
}
