package output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/rest-sh/restish/v2/internal/plugin"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

// PluginFormatter is an output.Formatter backed by a hook plugin. The plugin
// receives a short formatter session over CBOR on stdin. Its stdout is raw by
// default or framed when formatter.host_fallback is required.
type PluginFormatter struct {
	PluginPath   string
	FormatName   string
	PluginConfig json.RawMessage
	Context      context.Context
	Input        io.Reader
	Interactive  bool
	Timeout      time.Duration
	// HostFallback enables framed formatter output and delegation to the auto
	// formatter. It is set for plugins requiring formatter.host_fallback.
	HostFallback bool
}

var startPluginFormatterStream = func(ctx context.Context, path string, w io.Writer, in any) (formatterStream, error) {
	return plugin.StartFormatterStream(ctx, path, w, in)
}

var startPluginFormatterOutputStream = func(ctx context.Context, path string) (formatterOutputStream, error) {
	return plugin.StartFormatterOutputStream(ctx, path)
}

// Format sends the response to the plugin using the formatter session protocol
// and copies the plugin output to w.
func (f *PluginFormatter) Format(w io.Writer, resp *Response, color bool) error {
	if f.HostFallback {
		return f.formatWithHostFallback(w, resp, color)
	}
	stream, err := startPluginFormatterStream(f.context(), f.PluginPath, w, pluginwire.FormatterRequest{
		Type:         "formatter",
		Format:       f.FormatName,
		Color:        color,
		Event:        "start",
		PluginConfig: f.PluginConfig,
		Response: pluginwire.FormatterResponse{
			Proto:   resp.Proto,
			Status:  resp.Status,
			Headers: resp.Headers,
			Links:   resp.Links,
			Body:    resp.Body,
		},
	})
	if err != nil {
		return fmt.Errorf("formatter plugin %s: %w", f.FormatName, err)
	}
	if err := finishPluginFormatterSession(f.FormatName, stream, pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Event:  "end",
	}, f.Interactive, f.Input, w); err != nil {
		return err
	}
	return nil
}

// FormatValue renders a body/sub-value through a short formatter session,
// without implying that the value is a full HTTP response.
func (f *PluginFormatter) FormatValue(w io.Writer, value any, color bool) error {
	if f.HostFallback {
		return f.formatValueWithHostFallback(w, value, color)
	}
	stream, err := startPluginFormatterStream(f.context(), f.PluginPath, w, pluginwire.FormatterRequest{
		Type:         "formatter",
		Format:       f.FormatName,
		Color:        color,
		Event:        "start",
		PluginConfig: f.PluginConfig,
		Response:     pluginwire.FormatterResponse{},
	})
	if err != nil {
		return fmt.Errorf("formatter plugin %s: %w", f.FormatName, err)
	}
	if err := stream.Send(pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Event:  "item",
		Response: pluginwire.FormatterResponse{
			Body: value,
		},
	}); err != nil {
		return finishPluginFormatterSession(f.FormatName, stream, nil, false, nil, nil, err)
	}
	if err := finishPluginFormatterSession(f.FormatName, stream, pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Event:  "end",
	}, f.Interactive, f.Input, w); err != nil {
		return err
	}
	return nil
}

// StartValueStream starts a long-lived formatter plugin session.
func (f *PluginFormatter) StartValueStream(w io.Writer, base *Response, color bool) (ValueStream, error) {
	if f.HostFallback {
		return f.startValueStreamWithHostFallback(w, base, color)
	}
	stream, err := startPluginFormatterStream(f.context(), f.PluginPath, w, pluginwire.FormatterRequest{
		Type:         "formatter",
		Format:       f.FormatName,
		Color:        color,
		Event:        "start",
		PluginConfig: f.PluginConfig,
		Response: pluginwire.FormatterResponse{
			Proto:   base.Proto,
			Status:  base.Status,
			Headers: base.Headers,
			Links:   base.Links,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("formatter plugin %s: %w", f.FormatName, err)
	}
	return &pluginFormatterStream{
		pluginPath:  f.PluginPath,
		formatName:  f.FormatName,
		stream:      stream,
		input:       f.Input,
		output:      w,
		interactive: f.Interactive,
	}, nil
}

func finishPluginFormatterSession(formatName string, stream formatterStream, final any, interactive bool, input io.Reader, output io.Writer, priorErr ...error) error {
	var sendErr error
	if final != nil {
		sendErr = stream.Send(final)
	}
	var closeErr error
	if errors.Join(append(priorErr, sendErr)...) == nil && interactive {
		closeErr = stream.Interact(input, output)
	} else {
		closeErr = stream.Close()
	}
	if joined := errors.Join(append(priorErr, sendErr, closeErr)...); joined != nil {
		return fmt.Errorf("formatter plugin %s: %w", formatName, joined)
	}
	return nil
}

func (f *PluginFormatter) context() context.Context {
	if f.Context != nil {
		return f.Context
	}
	return context.Background()
}

type pluginFormatterStream struct {
	pluginPath  string
	formatName  string
	stream      formatterStream
	input       io.Reader
	output      io.Writer
	interactive bool
}

type formatterStream interface {
	Send(any) error
	Close() error
	Interact(io.Reader, io.Writer) error
}

type formatterOutputStream interface {
	SendWithTimeout(any, time.Duration) error
	Receive(*pluginwire.FormatterOutput) error
	Abort() error
	Close() error
	Interact(io.Reader, io.Writer) error
}

func (s *pluginFormatterStream) WriteValue(value any) error {
	if err := s.stream.Send(pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: s.formatName,
		Event:  "item",
		Response: pluginwire.FormatterResponse{
			Body: value,
		},
	}); err != nil {
		return fmt.Errorf("formatter plugin %s: %w", s.formatName, err)
	}
	return nil
}

func (s *pluginFormatterStream) Close() error {
	return finishPluginFormatterSession(s.formatName, s.stream, pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: s.formatName,
		Event:  "end",
	}, s.interactive, s.input, s.output)
}

const defaultFormatterOutputTimeout = 30 * time.Second

type formatterOutputReply struct {
	requestID uint64
	fallback  bool
}

type synchronizedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *synchronizedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

type pluginFormatterOutputSession struct {
	formatName  string
	stream      formatterOutputStream
	w           *synchronizedWriter
	fallback    ValueStream
	timeout     time.Duration
	replies     chan formatterOutputReply
	readerDone  chan struct{}
	stateMu     sync.Mutex
	nextID      uint64
	activeID    uint64
	terminal    error
	input       io.Reader
	output      io.Writer
	interactive bool
}

func newPluginFormatterOutputSession(formatName string, stream formatterOutputStream, w io.Writer, timeout time.Duration) *pluginFormatterOutputSession {
	if timeout <= 0 {
		timeout = defaultFormatterOutputTimeout
	}
	s := &pluginFormatterOutputSession{
		formatName: formatName,
		stream:     stream,
		w:          &synchronizedWriter{w: w},
		timeout:    timeout,
		replies:    make(chan formatterOutputReply, 1),
		readerDone: make(chan struct{}),
	}
	go s.readOutput()
	return s
}

func (s *pluginFormatterOutputSession) fail(err error) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.terminal == nil {
		s.terminal = err
	}
	s.activeID = 0
}

func (s *pluginFormatterOutputSession) terminalError() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.terminal
}

func (s *pluginFormatterOutputSession) complete(msg pluginwire.FormatterOutput) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.terminal != nil {
		return false
	}
	if s.activeID == 0 || msg.RequestID != s.activeID {
		s.terminal = fmt.Errorf("formatter output completed request %d, active request is %d", msg.RequestID, s.activeID)
		s.activeID = 0
		return false
	}
	s.activeID = 0
	return true
}

func (s *pluginFormatterOutputSession) readOutput() {
	defer close(s.readerDone)
	for {
		var msg pluginwire.FormatterOutput
		if err := s.stream.Receive(&msg); err != nil {
			s.fail(err)
			return
		}
		if msg.Type != pluginwire.MsgTypeFormatterOutput {
			s.fail(fmt.Errorf("unexpected formatter output type %q", msg.Type))
			return
		}
		if msg.Fallback && !msg.Complete {
			s.fail(fmt.Errorf("formatter fallback requires complete"))
			return
		}
		if len(msg.Data) > 0 {
			if _, err := s.w.Write(msg.Data); err != nil {
				s.fail(err)
				return
			}
		}
		if msg.Complete {
			if !s.complete(msg) {
				return
			}
			s.replies <- formatterOutputReply{requestID: msg.RequestID, fallback: msg.Fallback}
		}
	}
}

func (s *pluginFormatterOutputSession) beginRequest() (uint64, error) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.terminal != nil {
		return 0, s.terminal
	}
	if s.activeID != 0 {
		return 0, fmt.Errorf("formatter request %d is still active", s.activeID)
	}
	s.nextID++
	s.activeID = s.nextID
	return s.activeID, nil
}

func (s *pluginFormatterOutputSession) wait(requestID uint64) (bool, error) {
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case reply := <-s.replies:
		return s.validateReply(requestID, reply)
	case <-s.readerDone:
		select {
		case reply := <-s.replies:
			return s.validateReply(requestID, reply)
		default:
			return false, s.terminalError()
		}
	case <-timer.C:
		err := fmt.Errorf("formatter did not complete request %d within %s", requestID, s.timeout)
		s.fail(err)
		_ = s.stream.Abort()
		return false, err
	}
}

func (s *pluginFormatterOutputSession) validateReply(requestID uint64, reply formatterOutputReply) (bool, error) {
	if reply.requestID != requestID {
		return false, fmt.Errorf("formatter output completed request %d, want %d", reply.requestID, requestID)
	}
	return reply.fallback, nil
}

func (s *pluginFormatterOutputSession) exchange(req pluginwire.FormatterRequest) (bool, error) {
	requestID, err := s.beginRequest()
	if err != nil {
		return false, err
	}
	req.RequestID = requestID
	if err := s.stream.SendWithTimeout(req, s.timeout); err != nil {
		s.fail(err)
		return false, err
	}
	return s.wait(requestID)
}

func (s *pluginFormatterOutputSession) WriteValue(value any) error {
	fallback, err := s.exchange(pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: s.formatName,
		Event:  "item",
		Response: pluginwire.FormatterResponse{
			Body: value,
		},
	})
	if err != nil {
		return fmt.Errorf("formatter plugin %s: %w", s.formatName, err)
	}
	if fallback {
		err := s.fallback.WriteValue(value)
		if err != nil {
			s.fail(err)
		}
		return err
	}
	return nil
}

func (s *pluginFormatterOutputSession) closeProcess(prior ...error) error {
	var closeErr error
	if errors.Join(prior...) == nil && s.interactive {
		closeErr = s.stream.Interact(s.input, s.output)
	} else {
		closeErr = s.stream.Close()
	}
	<-s.readerDone
	terminalErr := s.terminalError()
	if errors.Is(terminalErr, io.EOF) {
		terminalErr = nil
	}
	return errors.Join(append(prior, closeErr, terminalErr)...)
}

func (s *pluginFormatterOutputSession) Close() error {
	var exchangeErr error
	if s.terminalError() == nil {
		fallback, err := s.exchange(pluginwire.FormatterRequest{
			Type:   "formatter",
			Format: s.formatName,
			Event:  "end",
		})
		exchangeErr = err
		if exchangeErr == nil && fallback {
			exchangeErr = fmt.Errorf("end cannot request fallback without a body")
		}
	} else {
		exchangeErr = s.terminalError()
	}
	var fallbackErr error
	if s.fallback != nil {
		fallbackErr = s.fallback.Close()
	}
	if err := s.closeProcess(exchangeErr, fallbackErr); err != nil {
		return fmt.Errorf("formatter plugin %s: %w", s.formatName, err)
	}
	return nil
}

func (f *PluginFormatter) startOutputSession(w io.Writer, req pluginwire.FormatterRequest) (*pluginFormatterOutputSession, bool, error) {
	stream, err := startPluginFormatterOutputStream(f.context(), f.PluginPath)
	if err != nil {
		return nil, false, fmt.Errorf("formatter plugin %s: %w", f.FormatName, err)
	}
	session := newPluginFormatterOutputSession(f.FormatName, stream, w, f.Timeout)
	session.input, session.output, session.interactive = f.Input, w, f.Interactive
	req.PluginConfig = f.PluginConfig
	fallback, err := session.exchange(req)
	if err != nil {
		return nil, false, fmt.Errorf("formatter plugin %s: %w", f.FormatName, session.closeProcess(err))
	}
	return session, fallback, nil
}

func (f *PluginFormatter) formatWithHostFallback(w io.Writer, resp *Response, color bool) error {
	session, fallback, err := f.startOutputSession(w, pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Color:  color,
		Event:  "start",
		Response: pluginwire.FormatterResponse{
			Proto:   resp.Proto,
			Status:  resp.Status,
			Headers: resp.Headers,
			Links:   resp.Links,
			Body:    resp.Body,
		},
	})
	if err != nil {
		return err
	}
	if fallback {
		if err := (&AutoFormatter{}).Format(session.w, resp, autoFallbackColor(w, resp, color)); err != nil {
			return session.closeProcess(err)
		}
	}
	return session.Close()
}

func (f *PluginFormatter) formatValueWithHostFallback(w io.Writer, value any, color bool) error {
	session, fallback, err := f.startOutputSession(w, pluginwire.FormatterRequest{
		Type:     "formatter",
		Format:   f.FormatName,
		Color:    color,
		Event:    "start",
		Response: pluginwire.FormatterResponse{},
	})
	if err != nil {
		return err
	}
	if fallback {
		return session.closeProcess(fmt.Errorf("formatter plugin %s: start cannot request fallback without a body", f.FormatName))
	}
	fallback, exchangeErr := session.exchange(pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Event:  "item",
		Response: pluginwire.FormatterResponse{
			Body: value,
		},
	})
	if exchangeErr != nil {
		return session.closeProcess(exchangeErr)
	}
	if fallback {
		if err := (&AutoFormatter{}).Format(session.w, &Response{Body: value}, color); err != nil {
			return session.closeProcess(err)
		}
	}
	return session.Close()
}

func (f *PluginFormatter) startValueStreamWithHostFallback(w io.Writer, base *Response, color bool) (ValueStream, error) {
	session, fallback, err := f.startOutputSession(w, pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Color:  color,
		Event:  "start",
		Response: pluginwire.FormatterResponse{
			Proto:   base.Proto,
			Status:  base.Status,
			Headers: base.Headers,
			Links:   base.Links,
		},
	})
	if err != nil {
		return nil, err
	}
	if fallback {
		return nil, session.closeProcess(fmt.Errorf("formatter plugin %s: start cannot request fallback without a body", f.FormatName))
	}
	session.fallback, err = (&AutoFormatter{}).StartValueStream(session.w, base, color)
	if err != nil {
		return nil, session.closeProcess(err)
	}
	return session, nil
}

func autoFallbackColor(w io.Writer, resp *Response, color bool) bool {
	return color || (resp != nil && IsTerminal(w) && strings.HasPrefix(Header(resp.Headers, "Content-Type"), "image/"))
}
