package bot

import (
	"fmt"
	"strings"

	"github.com/asalkeld/scrumpolice/scrum"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

// HandleMessage handle a received message for scrums and returns if the bot shall continue to process the message or stop
// continue = true
// stop = false
func (b *Bot) HandleScrumMessage(event *slack.MessageEvent) bool {
	// "start [team] [date]"
	// [team == first and only team]
	// [date == yesterday]
	// starting scrum for team [team] date [date]. if you want to abort say quit

	// this module only takes case in private messages
	if event.Channel[0] != 'D' {
		return true
	}

	if strings.HasPrefix(strings.ToLower(event.Text), "start") {
		return b.startScrum(event, false)
	}

	if strings.HasPrefix(strings.ToLower(event.Text), "skip") {
		return b.startScrum(event, true)
	}

	if strings.ToLower(event.Text) == "restart" || strings.ToLower(event.Text) == "quit" {
		return b.restartScrum(event)
	}

	return b.continueAnsweringQuestions(event)
}

func (b *Bot) restartScrum(event *slack.MessageEvent) bool {
	user, err := b.slackBotAPI.GetUserInfo(event.User)
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to get user information.")
		return false
	}

	us := b.scrum.GetUserState(user.Profile.DisplayName)
	tc := b.scrum.GetTeamForUser(us.User)
	if tc == nil {
		b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText("You're not part of a team, no point in doing a scrum report", true), slack.MsgOptionAsUser(true))
		return false
	}
	us.Answers = map[string]string{}
	us.Started = false
	b.scrum.SaveUserState(us)

	b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText("Your last report was deleted, you can `start` a new one again", true), slack.MsgOptionAsUser(true))
	return false
}

func (b *Bot) startScrum(event *slack.MessageEvent, isSkipped bool) bool {
	b.logSlackEvent(event, "startScrum")

	user, err := b.slackBotAPI.GetUserInfo(event.User)
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to get user information.")
		return false
	}

	us := b.scrum.GetUserState(user.Profile.DisplayName)
	tc := b.scrum.GetTeamForUser(us.User)
	if tc == nil {
		b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText("You're not part of a team, no point in doing a scrum report", true), slack.MsgOptionAsUser(true))
		return false
	}

	if isSkipped {
		us.Answers = map[string]string{}
		us.Started = false
		us.Skipped = true

		err = b.scrum.SaveUserState(us)
		if err != nil {
			b.logSlackRelatedError(event, err, "Fail to save userState.")
			return false
		}

		msg := fmt.Sprintf("Scrum report skipped for %s in team %s, type `restart` if it should not be skipped", us.User, tc.Name)
		b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText(msg, true), slack.MsgOptionAsUser(true))
		return false
	}

	us.Answers = map[string]string{}
	us.Started = true
	us.Skipped = false
	err = b.scrum.SaveUserState(us)
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to save userState.")
		return false
	}

	msg := fmt.Sprintf("Scrum report started %s for team %s, type `quit` anytime to stop", us.User, tc.Name)
	b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText(msg, true), slack.MsgOptionAsUser(true))

	return b.answerQuestions(event, us, tc)
}

func (b *Bot) answerQuestions(event *slack.MessageEvent, us *scrum.UserState, tc *scrum.TeamConfig) bool {
	if len(us.Answers) == len(tc.Questions) {
		b.scrum.SaveUserState(us)
		b.slackBotAPI.PostMessage(event.Channel,
			slack.MsgOptionText("Thanks for your scrum report my :deer:! :bear: with us for the digest. :owl: see you later!\n If you want to start again just say `restart`", true),
			slack.MsgOptionAsUser(true))
		b.logger.WithFields(log.Fields{
			"user": us.User,
			"team": tc.Name,
		}).Info("All questions anwsered, entry saved.")
		return false
	}

	for _, qu := range tc.Questions {
		if _, answered := us.Answers[qu]; !answered {
			b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText(qu, true), slack.MsgOptionAsUser(true))
			break
		}
	}

	return false
}

func (b *Bot) continueAnsweringQuestions(event *slack.MessageEvent) bool {
	user, err := b.slackBotAPI.GetUserInfo(event.User)
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to get user information.")
		return false
	}

	us := b.scrum.GetUserState(user.Profile.DisplayName)
	if !us.Started {
		return true
	}

	tc := b.scrum.GetTeamForUser(us.User)
	if tc == nil {
		b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText("You're not part of a team, no point in doing a scrum report", true), slack.MsgOptionAsUser(true))
		return false
	}

	if len(us.Answers) == len(tc.Questions) {
		return true
	}

	if us.Answers == nil {
		us.Answers = map[string]string{}
	}

	for _, qu := range tc.Questions {
		if _, answered := us.Answers[qu]; !answered {
			us.Answers[qu] = event.Text
			b.scrum.SaveUserState(us)
			break
		}
	}

	return b.answerQuestions(event, us, tc)
}
