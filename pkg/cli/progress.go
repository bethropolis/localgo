package cli

import (
	"os"

	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

type MultiProgress struct {
	pool *mpb.Progress
}

func NewMultiProgress(totalFiles int64) *MultiProgress {
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

	return func(current int64) {
		bar.SetCurrent(current)
	}
}

func (mp *MultiProgress) Wait() {
	mp.pool.Wait()
}

func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen-3] + "..."
}
