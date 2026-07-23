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

	// Replace %-placeholders before passing to the shell
	hook := h.config.ExecHook
	hook = strings.ReplaceAll(hook, "%f", filePath)
	hook = strings.ReplaceAll(hook, "%n", fileName)
	hook = strings.ReplaceAll(hook, "%s", fmt.Sprintf("%d", fileSize))
	hook = strings.ReplaceAll(hook, "%a", senderAlias)
	hook = strings.ReplaceAll(hook, "%i", senderIP)

	go func() {
		h.logger.Infof("Running exec hook: %s", hook)
		var cmd *exec.Cmd
		if h.config.Shell != "" {
			if parts := strings.Fields(h.config.Shell); len(parts) > 0 {
				cmd = exec.Command(parts[0], append(parts[1:], hook)...)
			}
		} else if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", hook)
		} else {
			cmd = exec.Command("sh", "-c", hook)
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
