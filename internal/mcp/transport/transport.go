package transport

import (
	"bufio"
	"encoding/json"
	"io"
)

// Transport handles line-based JSON communication over readers and writers.
type Transport struct {
	in  *bufio.Reader
	out *bufio.Writer
	enc *json.Encoder
}

// New creates a new transport.
func New(in io.Reader, out io.Writer) *Transport {
	writer := bufio.NewWriter(out)

	return &Transport{
		in:  bufio.NewReader(in),
		out: writer,
		enc: json.NewEncoder(writer),
	}
}

// ReadMessage reads a single JSON message.
func (t *Transport) ReadMessage() ([]byte, error) {
	line, err := t.in.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	return line, nil
}

// WriteMessage writes a single JSON message.
func (t *Transport) WriteMessage(value any) error {
	if err := t.enc.Encode(value); err != nil {
		return err
	}

	return t.out.Flush()
}
