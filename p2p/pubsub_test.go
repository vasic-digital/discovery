package p2p

import (
	"bytes"
	"testing"
)

func TestTopicName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "helix/cell/default"},
		{"  ", "helix/cell/default"},
		{"EU-West", "helix/cell/eu-west"},
		{" us-east ", "helix/cell/us-east"},
		{"Cell1", "helix/cell/cell1"},
	}
	for _, c := range cases {
		if got := TopicName(c.in); got != c.want {
			t.Errorf("TopicName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTopicNameDistinct(t *testing.T) {
	// Different cells MUST map to different topics — this is what makes the
	// mutation test (publishing on the wrong topic) detectable.
	if TopicName("alpha") == TopicName("beta") {
		t.Fatal("distinct cells produced the same topic name")
	}
}

func TestMessageRoundTrip(t *testing.T) {
	orig := Message{
		From:    "12D3KooWExample",
		Seq:     42,
		TSUnixN: 1_700_000_000_000_000_000,
		Payload: []byte("hello cross-cell"),
	}
	b, err := EncodeMessage(orig)
	if err != nil {
		t.Fatalf("EncodeMessage: %v", err)
	}
	got, err := DecodeMessage(b)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}
	if got.From != orig.From || got.Seq != orig.Seq || got.TSUnixN != orig.TSUnixN {
		t.Fatalf("header mismatch: got %+v want %+v", got, orig)
	}
	if !bytes.Equal(got.Payload, orig.Payload) {
		t.Fatalf("payload mismatch: got %q want %q", got.Payload, orig.Payload)
	}
}

func TestDecodeMessageError(t *testing.T) {
	if _, err := DecodeMessage([]byte("{not-json")); err == nil {
		t.Fatal("expected decode error on malformed JSON")
	}
}

func TestStdLoggerCapture(t *testing.T) {
	var buf bytes.Buffer
	l := NewStdLogger(&buf, "[test] ", true)
	l.Logf("event a=%d", 1)
	l.Logf("event b=%s", "x")
	lines := l.Lines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 captured lines, got %d", len(lines))
	}
	if !bytes.Contains(buf.Bytes(), []byte("[test] event a=1")) {
		t.Fatalf("writer missing first line: %q", buf.String())
	}
}
