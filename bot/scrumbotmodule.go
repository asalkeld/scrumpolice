package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/asalkeld/scrumpolice/common"
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
	us.LastAnswerDate = ""
	us.Started = false
	b.scrum.SaveUserState(us)

	b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText("Your last report was deleted, you can `start` a new one again", true), slack.MsgOptionAsUser(true))
	return false
}

func workItemFromMessage(m *slack.Message) string {
	prefix := map[string]string{
		"Pull request merged by":           "Merged PR",
		"Pull request ready for review by": "Marked PR ready for review",
		"Pull request opened by":           "Opened PR",
		"New issue created by":             "Created Issue",
		"Issue closed by":                  "Closed Issue",
	}
	a := m.Attachments[0]

	end := strings.Index(a.Title, "|")
	link := "link unknown"
	if end > 10 && end < len(a.Title) {
		link = a.Title[2:end]
	}

	for p, text := range prefix {
		if strings.HasPrefix(a.Pretext, p) {
			return "- " + text + " " + link
		}
	}
	return ""
}

func (b *Bot) WorkItemsForUser(ghUser string, since time.Time) ([]string, error) {
	workItems := []string{}
	chans, _, err := b.slackBotAPI.GetConversations(&slack.GetConversationsParameters{})
	if err != nil {
		return nil, err
	}

	for _, c := range chans {
		if !strings.HasPrefix(c.Name, "git-") {
			continue
		}
		resp, err := b.slackBotAPI.GetConversationHistory(&slack.GetConversationHistoryParameters{
			ChannelID: c.ID,
			Limit:     100,
			Oldest:    fmt.Sprint(since.Unix()),
		})
		if err != nil {
			fmt.Println(err, "GetConversationHistory ", c.Name)
			continue
		}

		for _, m := range resp.Messages {
			if !(m.BotProfile != nil && m.BotProfile.Name == "GitHub" && len(m.Attachments) > 0) {
				continue
			}
			if !strings.Contains(m.Attachments[0].Fallback, ghUser) {
				continue
			}
			wi := workItemFromMessage(&m)
			if wi != "" {
				workItems = append(workItems, wi)
			}
		}
	}
	return workItems, nil
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
		b.slackBotAPI.PostMessage("@"+event.User, slack.MsgOptionText("You're not part of a team, no point in doing a scrum report", true), slack.MsgOptionAsUser(true))
		return false
	}

	today, err := common.ToDay(tc.Timezone)
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to get current date.")
		return false
	}

	if isSkipped {
		us.Answers = map[string]string{}
		us.LastAnswerDate = today
		us.Started = false
		us.Skipped = true

		err = b.scrum.SaveUserState(us)
		if err != nil {
			b.logSlackRelatedError(event, err, "Fail to save userState.")
			return false
		}

		msg := fmt.Sprintf("Scrum report skipped for %s in team %s, type `restart` if it should not be skipped", us.User, tc.Name)
		b.slackBotAPI.PostMessage("@"+event.User, slack.MsgOptionText(msg, true), slack.MsgOptionAsUser(true))
		return false
	}

	us.Answers = map[string]string{}
	us.LastAnswerDate = today
	us.Started = true
	us.Skipped = false
	err = b.scrum.SaveUserState(us)
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to save userState.")
		return false
	}

	msg := fmt.Sprintf("Scrum report started %s for team %s, type `quit` anytime to stop", us.User, tc.Name)
	b.slackBotAPI.PostMessage("@"+event.User, slack.MsgOptionText(msg, true), slack.MsgOptionAsUser(true))

	if us.GithubUser != "" {
		now := time.Now()
		loc, err := time.LoadLocation(strings.TrimSpace(tc.Timezone))
		if err != nil {
			fmt.Println(err, "Failed to load timezone ", tc.Timezone)
			now.Add(-10 * time.Hour) // temp hack
		} else {
			now = now.In(loc)
		}
		yesterday := now.Add(-24 * time.Hour)
		if now.Weekday() == time.Monday {
			// need to include Friday's changes
			yesterday = now.Add(-3 * 24 * time.Hour)
		}

		gitItems, err := b.WorkItemsForUser(us.GithubUser, yesterday)
		if err != nil {
			b.logSlackRelatedError(event, err, "WorkItemsForUser")
		} else if len(gitItems) > 0 {
			b.slackBotAPI.PostMessage("@"+event.User,
				slack.MsgOptionAttachments(slack.Attachment{
					Pretext: "To get you started, here is a list of work items you have done in the last day:",
					Text:    strings.Join(gitItems, "\n"),
				}),
				slack.MsgOptionAsUser(true))
		}
	}

	return b.answerQuestions(event, us, tc)
}

func (b *Bot) answerQuestions(event *slack.MessageEvent, us *scrum.UserState, tc *scrum.TeamConfig) bool {
	if len(us.Answers) == len(tc.Questions) {
		b.scrum.SaveUserState(us)
		b.slackBotAPI.PostMessage("@"+event.User,
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
			b.slackBotAPI.PostMessage("@"+event.User, slack.MsgOptionText(qu, true), slack.MsgOptionAsUser(true))
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
		b.slackBotAPI.PostMessage("@"+event.User,
			slack.MsgOptionText("You're not part of a team, no point in doing a scrum report", true),
			slack.MsgOptionAsUser(true))
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
