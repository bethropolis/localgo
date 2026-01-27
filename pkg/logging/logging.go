package logging

import (
	"io"
	"os"
	"github.com/sirupsen/logrus"
)

func Init(quiet bool) {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	
	if quiet {
		// Discard all logs - essentially zero CPU cost for logging
		logrus.SetOutput(io.Discard)
	} else {
		logrus.SetOutput(os.Stdout)
	}
	logrus.SetLevel(logrus.InfoLevel)
}