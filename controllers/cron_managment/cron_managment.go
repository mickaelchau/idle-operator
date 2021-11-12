package cronmanagment

import (
	"time"

	"github.com/sirupsen/logrus"

	cron "github.com/robfig/cron/v3"
)

func IsInIdleTimezone(startTime, duration string) (bool, error) {
	cronStartTime, err := cron.ParseStandard(startTime)
	if err != nil {
		logrus.Errorf("'startTime' is not in CRON format: %s", err)
		return false, err
	}
	timeDuration, err := time.ParseDuration(duration)
	if err != nil {
		logrus.Errorf("'duration' is not in CRON format: %s", err)
		return false, err
	}
	functionStartTime := time.Now()
	substractedDurationTime := functionStartTime.Add(-timeDuration) //Automaticly removes timeDuration
	previousIdlingStartTime := cronStartTime.Next(substractedDurationTime)
	logrus.Infof("IDLING STARTING THE: %d/%d/%d AT: %d:%d:%d", previousIdlingStartTime.Day(), previousIdlingStartTime.Month(),
		previousIdlingStartTime.Year(), previousIdlingStartTime.Hour(), previousIdlingStartTime.Minute(),
		previousIdlingStartTime.Second())
	isInsideTimezone := previousIdlingStartTime.Before(functionStartTime)
	return isInsideTimezone, nil
}
