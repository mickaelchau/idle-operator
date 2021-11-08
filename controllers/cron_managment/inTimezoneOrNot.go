package CronManagment

import (
	"time"

	"log"

	cron "github.com/robfig/cron/v3"
)

func InTimezoneOrNot(startTime string, duration string) (bool, error) {
	cronStartTime, err := cron.ParseStandard(startTime)
	if err != nil {
		log.Println(err)
		return false, err
	}
	parsedDuration, err := time.ParseDuration(duration)
	if err != nil {
		log.Println(err)
		return false, err
	}
	nowTime := time.Now()
	log.Println(nowTime.Day(), nowTime.Month(), nowTime.Year(), nowTime.Hour(), nowTime.Minute(), nowTime.Second())
	substractedDurationTime := nowTime.Add(-parsedDuration)
	prevIdlingTime := cronStartTime.Next(substractedDurationTime)
	log.Println(prevIdlingTime.Day(), prevIdlingTime.Month(), prevIdlingTime.Year(),
		prevIdlingTime.Hour(), prevIdlingTime.Minute(), prevIdlingTime.Second())
	pastTime := prevIdlingTime.Before(time.Now())
	return pastTime, nil
}
