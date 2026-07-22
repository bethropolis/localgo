package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func (h *ReceiveHandler) runExecHook(filePath, fileName, senderAlias, senderIP string, fileSize int64) {
	if h.config.ExecHook == "" {
		return
	}

	go func() {
		h.logger.Infof("Running exec hook: %s", h.config.ExecHook)
		var cmd *exec.Cmd
		if h.config.Shell != "" {
			if parts := strings.Fields(h.config.Shell); len(parts) > 0 {
				cmd = exec.Command(parts[0], append(parts[1:], h.config.ExecHook)...)
			}
		}
		if cmd == nil && runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", h.config.ExecHook)
		} else {
			cmd = exec.Command("sh", "-c", h.config.ExecHook)
		}
		cmd.Env = append(os.Environ(),
			"LOCALGO_FILE="+filePath,
			"LOCALGO_NAME="+fileName,
			fmt.Sprintf("LOCALGO_SIZE=%d", fileSize),
			"LOCALGO_ALIAS="+senderAlias,
			"LOCALGO_IP="+senderIP,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			h.logger.Errorf("Exec hook failed: %v, output: %s", err, string(output))
		} else {
			h.logger.Debugf("Exec hook completed, output: %s", string(output))
		}
	}()
}
