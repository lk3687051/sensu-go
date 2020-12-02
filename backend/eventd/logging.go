package eventd
import (
	"fmt"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"gopkg.in/natefinch/lumberjack.v2"
)
// Logger ...
type Logger interface {
	Stop()
	Println(v interface{})
}
var event_logger *lumberjack.Logger
// RawLogger ...
type RawLogger struct{
	path string
}

func CheckNeedLog(event *corev2.Event) bool {
	if event.HasCheck() {
		for _, handler := range event.Check.Handlers{
			if handler == "alarm" {
				return true
			}
		}
	}
	return false
}
// Println ...
func (l *RawLogger) Println(v interface{}) {
	if event_logger != nil {
		event := v.(*corev2.Event)
		if CheckNeedLog(event) {
			_, err := fmt.Fprintf(event_logger, "%d,%s,%s,%s,%d\n",event.Check.Executed, event.GetUUID().String(),event.Entity.Name,event.Check.Name,event.Check.Status)
			if err != nil {
				logger.WithError(err).Fatal("Unable write event log")
			}
		}
	} else {
		event_logger = &lumberjack.Logger{
			Filename:   l.path,
			MaxSize:    1000, // megabytes
			MaxBackups: 20,
			MaxAge:     30, //days
			Compress:   false, // disabled by default
		}
	}
}

// Stop ...
func (l *RawLogger) Stop() {
	if event_logger != nil {
		event_logger.Close()
		event_logger = nil
	}
}
