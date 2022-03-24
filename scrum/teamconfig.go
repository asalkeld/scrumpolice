package scrum

import (
	"strings"
	"time"

	"github.com/asalkeld/scrumpolice/common"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/robfig/cron"
	"github.com/slack-go/slack"
)

func (tc *TeamConfig) ReadyToSendReport() (bool, error) {
	now, err := common.NowWithLocation(tc.Timezone)
	if err != nil {
		return false, err
	}

	c, err := cron.ParseStandard(tc.ReportScheduleCron)
	if err != nil {
		return false, err
	}

	newLayout := "15:04"
	startOfDay, err := time.Parse(newLayout, "01:00")
	if err != nil {
		return false, err
	}

	scheduledTime := c.Next(startOfDay)

	return now.After(scheduledTime), nil
}

func (tc *TeamConfig) GenerateReport(today string, members []*UserState) ([]slack.Attachment, []string) {
	attachments := []slack.Attachment{}
	didNotDoReport := []string{}
	outOfOffice := []string{}

	for _, member := range members {
		answers := member.Answers
		if len(answers) > 0 {
			// don't keep reusing previous answers.
			if member.LastAnswerDate != "" && member.LastAnswerDate != today {
				answers = map[string]string{}
			}
		}

		if member.OutOfOffice {
			outOfOffice = append(outOfOffice, member.User)
		} else if len(answers) == 0 {
			didNotDoReport = append(didNotDoReport, "@"+member.User)
		} else if member.Skipped {
			attachment := slack.Attachment{
				Color:      colorful.FastHappyColor().Hex(),
				MarkdownIn: []string{"text", "pretext"},
				Pretext:    "@" + member.User,
				Text:       "Has nothing to declare.",
			}
			attachments = append(attachments, attachment)
		} else {
			message := ""
			for idx, q := range tc.Questions {
				message += q + "\n" + answers[q]

				if idx < len(tc.Questions)-1 {
					message += "\n\n"
				}
			}

			attachment := slack.Attachment{
				Color:      colorful.FastHappyColor().Hex(),
				MarkdownIn: []string{"text", "pretext"},
				Pretext:    "@" + member.User,
				Text:       message,
			}
			attachments = append(attachments, attachment)
		}
	}

	if len(outOfOffice) > 0 {
		persons := outOfOffice[0]
		verb := "is"

		if len(outOfOffice) > 1 {
			persons = strings.Join(outOfOffice[0:(len(outOfOffice)-2)], ", ") + " and " + outOfOffice[(len(outOfOffice)-1)]
			verb = "are"
		}

		attachment := slack.Attachment{
			Color:      colorful.FastHappyColor().Hex(),
			MarkdownIn: []string{"text", "pretext"},
			Pretext:    "Currently out of office",
			Text:       persons + " " + verb + " currently out of office :sunglasses: :palm_tree:",
		}

		attachments = append(attachments, attachment)
	}

	return attachments, didNotDoReport
}
