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
	idlingStartTime := cronStartTime.Next(substractedDurationTime)
	logrus.Infof("Idling start the: %d/%d/%d at: %d:%d:%d", idlingStartTime.Day(), idlingStartTime.Month(),
		idlingStartTime.Year(), idlingStartTime.Hour(), idlingStartTime.Minute(),
		idlingStartTime.Second())
	idlingEndTime := idlingStartTime.Add(timeDuration)
	logrus.Infof("Idling end the: %d/%d/%d at: %d:%d:%d", idlingEndTime.Day(), idlingEndTime.Month(),
		idlingEndTime.Year(), idlingEndTime.Hour(), idlingEndTime.Minute(),
		idlingEndTime.Second())
	isInsideTimezone := idlingStartTime.Before(functionStartTime)
	return isInsideTimezone, nil
}
