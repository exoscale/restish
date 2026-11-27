package plugin

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestFormatterTerminalCtrlCInterruptsHost(t *testing.T) {
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if _, err := writer.Write([]byte{3}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var messages bytes.Buffer
	stream := &FormatterStream{path: "test", stdin: nopWriteCloser{Writer: &messages}}
	if err := stream.forwardTerminalInput(input, make(chan struct{})); !errors.Is(err, ErrTerminalInterrupt) {
		t.Fatalf("Ctrl-C error = %v, want ErrTerminalInterrupt", err)
	}
	if messages.Len() != 0 {
		t.Fatal("Ctrl-C was forwarded to the formatter")
	}
}
