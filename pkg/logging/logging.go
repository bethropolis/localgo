package logging

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

func Init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
}

func SetQuiet() {
	logrus.SetOutput(io.Discard)
}