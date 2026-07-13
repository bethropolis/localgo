package send

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"go.uber.org/zap"
)

func uploadFile(ctx context.Context, client *http.Client, device *model.Device, filePath, fileID, sessionID, token, scheme string, trackProgress func(int64), logger *zap.SugaredLogger) error {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s://%s/api/localsend/v2/upload?sessionId=%s&fileId=%s&token=%s", scheme, net.JoinHostPort(device.IP, strconv.Itoa(device.Port)), sessionID, fileID, token)

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file stats: %w", err)
	}

	var body io.ReadCloser = file
	if trackProgress != nil {
		bar := &progressBar{current: 0, track: trackProgress}
		body = &progressTracker{Reader: file, Closer: file, bar: bar}
	}

	// Wrap with idle timeout: cancel request if no data flows for 15s
	uploadCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	body = NewIdleTimeoutReader(body, 15*time.Second, cancel)

	req, err := http.NewRequestWithContext(uploadCtx, http.MethodPost, url, body)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = stat.Size()

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("upload stalled: no data transmitted for 15s")
		}
		return fmt.Errorf("failed to send upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload request failed with status: %s", resp.Status)
	}

	return nil
}

// IdleTimeoutReader wraps an io.ReadCloser and cancels the context if no data
// is read within the configured idle duration.
type IdleTimeoutReader struct {
	r           io.ReadCloser
	idleTimeout time.Duration
	timer       *time.Timer
	cancel      func()
}

// NewIdleTimeoutReader creates an IdleTimeoutReader.
func NewIdleTimeoutReader(r io.ReadCloser, timeout time.Duration, cancel func()) *IdleTimeoutReader {
	tr := &IdleTimeoutReader{
		r:           r,
		idleTimeout: timeout,
		cancel:      cancel,
	}
	tr.timer = time.AfterFunc(timeout, func() {
		tr.cancel()
	})
	return tr
}

func (tr *IdleTimeoutReader) Read(p []byte) (int, error) {
	tr.timer.Reset(tr.idleTimeout)
	n, err := tr.r.Read(p)
	if err != nil {
		tr.timer.Stop()
	}
	return n, err
}

func (tr *IdleTimeoutReader) Close() error {
	tr.timer.Stop()
	return tr.r.Close()
}

type progressBar struct {
	current int64
	track   func(int64)
}

type progressTracker struct {
	io.Reader
	io.Closer
	bar *progressBar
}

func (pt *progressTracker) Read(p []byte) (int, error) {
	n, err := pt.Reader.Read(p)
	if n > 0 && pt.bar != nil {
		pt.bar.current += int64(n)
		pt.bar.track(pt.bar.current)
	}
	return n, err
}
