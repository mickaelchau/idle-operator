package main

import (
	"fmt"
	"time"

	cron "github.com/robfig/cron/v3"
)

func main() {
	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := specParser.Parse("0 16 * * 5-6,0")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("sched:", sched.Next(time.Now()))
}
