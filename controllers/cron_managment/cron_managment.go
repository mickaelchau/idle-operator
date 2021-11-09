package cronmanagment

import (
	"log"
	"time"

	"github.com/sirupsen/logrus"

	cron "github.com/robfig/cron/v3"
)

func IsInTimezone(startTime, duration string) (bool, error) {
	cronStartTime, err := cron.ParseStandard(startTime)
	if err != nil {
		log.Fatalln("'startTime' is not in CRON format", err)
		return false, err
	}
	timeDuration, err := time.ParseDuration(duration)
	if err != nil {
		log.Fatalln("'duration' is not in CRON format", err)
		return false, err
	}
	functionStartTime := time.Now()
	logrus.Info(functionStartTime.Day(), functionStartTime.Month(), functionStartTime.Year(), functionStartTime.Hour(),
		functionStartTime.Minute(), functionStartTime.Second())
	substractedDurationTime := functionStartTime.Add(-timeDuration) //Automaticly removes timeDuration
	previousIdlingStartTime := cronStartTime.Next(substractedDurationTime)
	logrus.Infoln("IDLING STARTING AT:", previousIdlingStartTime.Day(), "= DAY |", previousIdlingStartTime.Month(), "= MONTH |",
		previousIdlingStartTime.Year(), "= YEAR |", previousIdlingStartTime.Hour(), "= HOUR |", previousIdlingStartTime.Minute(),
		"= MINUTES |", previousIdlingStartTime.Second(), "= SECONDS")
	isInsideTimezone := previousIdlingStartTime.Before(functionStartTime)
	return isInsideTimezone, nil
}
