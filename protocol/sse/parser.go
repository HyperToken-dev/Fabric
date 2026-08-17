package sse

import (
	"bytes"
	"strings"
)

// Event is one parsed Server-Sent Events message.
//
// Event stores the optional SSE event name, Data stores joined data lines using
// the SSE-required newline separator, and Raw stores the exact bytes consumed for
// this event. Raw is preserved so proxy code can pass through unmodified events
// without changing provider-specific formatting.
type Event struct {
	Event string
	Data  string
	Raw   []byte
}

// Parser incrementally parses SSE events from arbitrary network chunks.
//
// Concurrency: Parser owns mutable buffers and is not safe for concurrent use.
// A Parser instance should be used for exactly one ordered stream.
type Parser struct {
	line  bytes.Buffer
	event eventBuilder
}

// eventBuilder accumulates one in-progress SSE event until a blank line closes it.
type eventBuilder struct {
	event string
	data  bytes.Buffer
	raw   bytes.Buffer
}

// Write consumes a chunk of SSE bytes and returns every complete event produced
// by that chunk.
//
// The chunk may start or end in the middle of a line. Incomplete lines are held
// internally until a later Write or Finish call supplies the line ending.
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

// Finish flushes any buffered line and returns the final event if the stream
// ended without a trailing blank line.
//
// Callers should invoke Finish once when the upstream stream reaches EOF. Finish
// does not validate provider payload JSON; it only completes SSE framing.
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

// consumeLine updates parser state with one complete SSE line.
//
// Comment lines are retained in Raw for transparent pass-through but do not
// affect Event or Data. Unknown fields are also preserved only in Raw.
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

// finish snapshots the accumulated event and detaches Raw from the builder buffer.
func (b *eventBuilder) finish() Event {
	return Event{
		Event: b.event,
		Data:  b.data.String(),
		Raw:   append([]byte(nil), b.raw.Bytes()...),
	}
}

// DataEquals compares event data after trimming surrounding whitespace.
//
// This is mainly used for protocol sentinels such as [DONE], where providers may
// include incidental whitespace around the data value.
func (e Event) DataEquals(value string) bool {
	return strings.TrimSpace(e.Data) == value
}

// FormatData serializes one SSE data event.
//
// event is omitted when empty. data is written as a single data line and should
// already contain a JSON payload or protocol sentinel suitable for the caller's
// stream format.
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
