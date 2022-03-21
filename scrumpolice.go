package main

import (
	_ "embed"
	"fmt"
	"log"
	"time"

	"github.com/asalkeld/scrumpolice/bot"
	"github.com/asalkeld/scrumpolice/common"
	"github.com/asalkeld/scrumpolice/scrum"
	"github.com/nitrictech/go-sdk/faas"
	"github.com/nitrictech/go-sdk/resources"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

const (
	Version = "0.0.1"
)

//go:embed .token
var slackBotToken string

func main() {
	fmt.Println("Version", Version)
	fmt.Println("")

	logger := logrus.New()
	if slackBotToken == "" {
		log.Fatalln("slack bot token must be set in .token file")
	}

	slackAPIClient := slack.New(slackBotToken)
	spApi := resources.NewApi("scrumpolice")
	sc := scrum.NewConfig()
	ss := scrum.NewService(sc, slackAPIClient)
	b := bot.New(slackAPIClient, logger, ss)

	err := resources.NewSchedule("sendReport", "30 minutes", func(ec *faas.EventContext, next faas.EventHandler) (*faas.EventContext, error) {
		fmt.Println("got scheduled event ", string(ec.Request.Data()))

		sc.ReloadAndDistributeChange()
		for _, tc := range sc.Config().Teams {
			now, err := common.NowWithLocation(tc.Timezone)
			if err != nil {
				fmt.Println(err)
				continue
			}

			lastCheck := now.Add(-30 * time.Minute)
			c, err := cron.ParseStandard(tc.ReportScheduleCron)
			if err != nil {
				fmt.Println(err)
				continue
			}
			nextRun := c.Next(lastCheck)
			if nextRun.Before(*now) && nextRun.After(lastCheck) {
				fmt.Println("run report now!")
			}
		}

		return next(ec)
	})
	if err != nil {
		log.Fatalln(err)
	}

	sc.ReloadAndDistributeChange()

	spApi.Post("/events", b.EventHandler)

	spApi.Post("/config", sc.PostHandler)
	spApi.Get("/config", sc.ListHandler)
	spApi.Delete("/config/:name", sc.DeleteHandler)
	spApi.Get("/config/:name", sc.GetHandler)
	spApi.Put("/config/:name", sc.PutHandler)

	err = resources.Run()
	if err != nil {
		panic(err)
	}
}
