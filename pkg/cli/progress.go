package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
)

// NewFileProgressBar creates a new progress bar for file transfers.
func NewFileProgressBar(filename string, size int64) *progressbar.ProgressBar {
	desc := filename
	if len(desc) > 30 {
		desc = desc[:27] + "..."
	}
	return progressbar.NewOptions64(
		size,
		progressbar.OptionSetDescription(desc),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(20),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetRenderBlankState(true),
	)
}
