package sse

import (
	"strings"
	"testing"
)

func TestParserEmitsEventsSplitAcrossChunks(t *testing.T) {
	var parser Parser
	events, err := parser.Write([]byte("event: message\ndata: {\"choices\""))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %d, want 0", len(events))
	}
	events, err = parser.Write([]byte(":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].Event != "message" {
		t.Fatalf("event = %q, want message", events[0].Event)
	}
	if !strings.Contains(events[0].Data, `"content":"hi"`) {
		t.Fatalf("data = %q", events[0].Data)
	}
}

func TestParserHandlesCommentsAndDone(t *testing.T) {
	var parser Parser
	events, err := parser.Write([]byte(": keep-alive\n\ndata: [DONE]\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].Data != "" {
		t.Fatalf("comment event data = %q, want empty", events[0].Data)
	}
	if !events[1].DataEquals("[DONE]") {
		t.Fatal("second event should be DONE")
	}
}

func TestParserJoinsMultilineDataAndHandlesCRLF(t *testing.T) {
	var parser Parser
	events, err := parser.Write([]byte("data: first\r\ndata: second\r\n\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].Data != "first\nsecond" {
		t.Fatalf("data = %q, want joined multiline data", events[0].Data)
	}
}

func TestParserFinishEmitsPartialEvent(t *testing.T) {
	var parser Parser
	if _, err := parser.Write([]byte("data: partial")); err != nil {
		t.Fatal(err)
	}
	events, err := parser.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Data != "partial" {
		t.Fatalf("events = %+v, want partial event", events)
	}
}

func TestFormatData(t *testing.T) {
	got := string(FormatData("message", []byte(`{"ok":true}`)))
	want := "event: message\ndata: {\"ok\":true}\n\n"
	if got != want {
		t.Fatalf("formatted = %q, want %q", got, want)
	}
}
