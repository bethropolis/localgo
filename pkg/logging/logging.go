
package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Init initializes the logger with a structured format.
func Init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
}
