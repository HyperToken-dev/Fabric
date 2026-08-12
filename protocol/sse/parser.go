package sse

import (
	"bytes"
	"strings"
)

type Event struct {
	Event string
	Data  string
	Raw   []byte
}

type Parser struct {
	line  bytes.Buffer
	event eventBuilder
}

type eventBuilder struct {
	event string
	data  bytes.Buffer
	raw   bytes.Buffer
}

func (p *Parser) Write(chunk []byte) ([]Event, error) {
	var events []Event
	for len(chunk) > 0 {
		idx := bytes.IndexByte(chunk, '\n')
		if idx < 0 {
			_, err := p.line.Write(chunk)
			return events, err
		}

		if _, err := p.line.Write(chunk[:idx+1]); err != nil {
			return events, err
		}
		lineBytes := append([]byte(nil), p.line.Bytes()...)
		p.line.Reset()

		event, ok, err := p.consumeLine(lineBytes)
		if err != nil {
			return events, err
		}
		if ok {
			events = append(events, event)
		}
		chunk = chunk[idx+1:]
	}
	return events, nil
}

func (p *Parser) Finish() ([]Event, error) {
	if p.line.Len() > 0 {
		lineBytes := append([]byte(nil), p.line.Bytes()...)
		p.line.Reset()
		if !bytes.HasSuffix(lineBytes, []byte("\n")) {
			lineBytes = append(lineBytes, '\n')
		}
		if _, _, err := p.consumeLine(lineBytes); err != nil {
			return nil, err
		}
	}
	if p.event.raw.Len() == 0 {
		return nil, nil
	}
	event := p.event.finish()
	p.event = eventBuilder{}
	return []Event{event}, nil
}

func (p *Parser) consumeLine(lineBytes []byte) (Event, bool, error) {
	line := strings.TrimSuffix(string(lineBytes), "\n")
	line = strings.TrimSuffix(line, "\r")
	if line == "" {
		if _, err := p.event.raw.Write(lineBytes); err != nil {
			return Event{}, false, err
		}
		event := p.event.finish()
		p.event = eventBuilder{}
		return event, true, nil
	}
	if _, err := p.event.raw.Write(lineBytes); err != nil {
		return Event{}, false, err
	}
	if strings.HasPrefix(line, ":") {
		return Event{}, false, nil
	}

	field, value, found := strings.Cut(line, ":")
	if !found {
		field = line
		value = ""
	} else if strings.HasPrefix(value, " ") {
		value = strings.TrimPrefix(value, " ")
	}

	switch field {
	case "event":
		p.event.event = value
	case "data":
		if p.event.data.Len() > 0 {
			if err := p.event.data.WriteByte('\n'); err != nil {
				return Event{}, false, err
			}
		}
		_, err := p.event.data.WriteString(value)
		return Event{}, false, err
	}
	return Event{}, false, nil
}

func (b *eventBuilder) finish() Event {
	return Event{
		Event: b.event,
		Data:  b.data.String(),
		Raw:   append([]byte(nil), b.raw.Bytes()...),
	}
}

func (e Event) DataEquals(value string) bool {
	return strings.TrimSpace(e.Data) == value
}

func FormatData(event string, data []byte) []byte {
	var out bytes.Buffer
	if event != "" {
		_, _ = out.WriteString("event: ")
		_, _ = out.WriteString(event)
		_, _ = out.WriteString("\n")
	}
	_, _ = out.WriteString("data: ")
	_, _ = out.Write(data)
	_, _ = out.WriteString("\n\n")
	return out.Bytes()
}
