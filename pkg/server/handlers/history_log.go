package handlers

import (
	"github.com/bethropolis/localgo/pkg/history"
)

func (h *ReceiveHandler) logTransfer(senderAlias, senderIP, fileName, filePath string, size int64, fileType, status string) {
	if h.historyLog == nil {
		return
	}
	entry := history.Entry{
		SenderAlias: senderAlias,
		SenderIP:    senderIP,
		FileName:    fileName,
		FilePath:    filePath,
		FileSize:    size,
		FileType:    fileType,
		Status:      status,
	}
	if err := h.historyLog.Log(entry); err != nil {
		h.logger.Errorf("Failed to log transfer history: %v", err)
	}
}
