package cli

import (
	"fmt"
	"os"
	"sync"

	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

type MultiProgress struct {
	pool *mpb.Progress
	bars []*mpb.Bar
	mu   sync.Mutex
}

func NewMultiProgress(_ int64) *MultiProgress {
	return &MultiProgress{
		pool: mpb.New(
			mpb.WithOutput(os.Stderr),
		),
	}
}

func (mp *MultiProgress) AddBar(name string, size int64) func(int64) {
	if size == 0 {
		return func(int64) {}
	}

	bar := mp.pool.AddBar(size,
		mpb.PrependDecorators(
			decor.Name(truncateName(name, 30), decor.WC{W: 32, C: decor.DidentRight}),
			decor.CountersKibiByte("% 11.2f / % 11.2f"),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WC{W: 5}),
		),
	)
	bar.EnableTriggerComplete()

	mp.mu.Lock()
	mp.bars = append(mp.bars, bar)
	mp.mu.Unlock()

	return func(current int64) {
		bar.SetCurrent(current)
	}
}

func (mp *MultiProgress) ForceComplete() {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	for _, bar := range mp.bars {
		bar.SetTotal(0, true)
	}
}

func (mp *MultiProgress) Wait() {
	mp.pool.Wait()

	mp.mu.Lock()
	barsRendered := len(mp.bars)
	mp.mu.Unlock()

	// Clear only the lines with actual rendered progress bars
	for i := 0; i < barsRendered; i++ {
		fmt.Fprintf(os.Stderr, "\033[F\033[K")
	}
	fmt.Fprintf(os.Stderr, "%s Files transferred successfully\n", IconCheck)
}

func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen-3] + "..."
}
