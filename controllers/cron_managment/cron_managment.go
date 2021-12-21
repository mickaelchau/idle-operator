package cronmanagment

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"

	cron "github.com/robfig/cron/v3"
)

func IsInIdleTimezone(startTime, duration string, logger logr.Logger) (bool, error) {
	cronStartTime, err := cron.ParseStandard(startTime)
	if err != nil {
		logger.Error(err, "'startTime' is not in CRON format")
		return false, err
	}
	timeDuration, err := time.ParseDuration(duration)
	if err != nil {
		logger.Error(err, "'duration' is not in CRON format: %s")
		return false, err
	}
	functionStartTime := time.Now()
	substractedDurationTime := functionStartTime.Add(-timeDuration) //Automaticly removes timeDuration
	idlingStartTime := cronStartTime.Next(substractedDurationTime)
	logger.Info(fmt.Sprintf(
		"Idling start the: %d/%d/%d at: %dh:%dm:%ds",
		idlingStartTime.Day(),
		idlingStartTime.Month(),
		idlingStartTime.Year(),
		idlingStartTime.Hour(),
		idlingStartTime.Minute(),
		idlingStartTime.Second()))
	idlingEndTime := idlingStartTime.Add(timeDuration)
	logger.Info(fmt.Sprintf(
		"Idling start the: %d/%d/%d at: %dh:%dm:%ds",
		idlingEndTime.Day(),
		idlingEndTime.Month(),
		idlingEndTime.Year(),
		idlingEndTime.Hour(),
		idlingEndTime.Minute(),
		idlingEndTime.Second()))
	isInsideTimezone := idlingStartTime.Before(functionStartTime)
	return isInsideTimezone, nil
}
