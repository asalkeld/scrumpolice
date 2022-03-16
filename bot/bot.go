package bot

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/asalkeld/scrumpolice/scrum"
	"github.com/slack-go/slack"
)

var (
	OutOfOfficeRegex, _ = regexp.Compile("^(.+) is out of office$")
)

type Bot struct {
	slackBotAPI *slack.Client

	scrum scrum.Service

	name string
	id   string

	logger *log.Logger
}

func New(slackApiClient *slack.Client, logger *log.Logger, scrum scrum.Service) *Bot {
	b := &Bot{
		slackBotAPI: slackApiClient,
		logger:      logger,
		scrum:       scrum,
		name:        "scrumpolice",
		id:          "u037kbftcgy",
	}

	return b
}

func (b *Bot) handleMessage(event *slack.MessageEvent, isIM bool) {
	if event.BotID != "" {
		if strings.HasPrefix(event.Text, "Scrum report started") {
			// this is from me, get the user id
			b.logger.Println("this looks like my message, user ", event.User, " botID ", event.BotID)
		}
		b.logger.Println("handleMessage SKIPPING msg from bot ", event)
		// Ignore the messages coming from other bots
		return
	}

	eventText := strings.ToLower(event.Text)

	// HANDLE GLOBAL PUBLIC COMMANDS HERE
	if strings.Contains(eventText, ":wave:") {
		b.reactToEvent(event, "wave")
		return
	}

	adressedToMe := b.adressedToMe(eventText)
	fmt.Println("addressed to me? ", adressedToMe)
	if !isIM && !adressedToMe {
		return
	}

	if !b.HandleScrumMessage(event) {
		return
	}

	// FROM HERE All Commands need to be adressed to me or handled in private conversations
	if !isIM && adressedToMe {
		eventText = b.trimBotNameInMessage(eventText)
	}

	// From here on i only care of messages that were clearly adressed to me so i'll just get out
	if !adressedToMe && !isIM {
		return
	}

	// Handle commands adressed to me (can be public or private)
	if eventText == "source code" {
		b.sourceCode(event)
		return
	}

	if strings.HasPrefix(eventText, "report-dm") {
		teamName := strings.TrimSpace(strings.TrimPrefix(event.Text, "report-dm"))
		b.sendReportDm(event, teamName, "@"+event.User)
		return
	}

	if strings.HasPrefix(eventText, "report") {
		teamName := strings.TrimSpace(strings.TrimPrefix(event.Text, "report"))
		b.sendReport(event, teamName)
		return
	}

	if eventText == "help" {
		b.help(event)
		return
	}

	if eventText == "tutorial" {
		b.tutorial(event)
		return
	}

	if eventText == "out of office" {
		b.outOfOffice(event, event.User)
		return
	}

	if OutOfOfficeRegex.MatchString(eventText) {
		b.outOfOffice(event, strings.Split(strings.Trim(eventText, " "), " ")[0])
		return
	}

	if eventText == "i'm back" || eventText == "i am back" {
		b.backInOffice(event)
		return
	}

	// Unrecognized message so let's help the user
	b.unrecognizedMessage(event)
}

func (b *Bot) adressedToMe(msg string) bool {
	fmt.Println("adressedToMe ", msg)
	fmt.Println(b.id, " ", b.name)
	return strings.HasPrefix(msg, strings.ToLower("<@"+b.id+">")) ||
		strings.HasPrefix(msg, strings.ToLower(b.name))
}

func (b *Bot) trimBotNameInMessage(msg string) string {
	fmt.Println("trimBotNameInMessage ", msg)
	msg = strings.Replace(msg, strings.ToLower("<@"+b.id+">"), "", 1)
	msg = strings.Replace(msg, strings.ToLower(b.name), "", 1)
	msg = strings.Trim(msg, " :\n")
	fmt.Println("trimBotNameInMessage ", msg)

	return msg
}

func (b *Bot) reactToEvent(event *slack.MessageEvent, reaction string) {
	item := slack.ItemRef{
		Channel:   event.Channel,
		Timestamp: event.Timestamp,
	}
	err := b.slackBotAPI.AddReaction(reaction, item)
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to add reaction to slack.")
		return
	}
}

func (b *Bot) sendReport(event *slack.MessageEvent, teamName string) {
	tc, err := b.scrum.GetTeamByName(teamName)
	if err != nil {
		b.logSlackRelatedError(event, err, "can't get team")
	}

	b.scrum.SendReportForTeam(tc, tc.Channel)
}

func (b *Bot) sendReportDm(event *slack.MessageEvent, teamName, sendTo string) {
	tc, err := b.scrum.GetTeamByName(teamName)
	if err != nil {
		b.logSlackRelatedError(event, err, "can't get team")
	}

	b.scrum.SendReportForTeam(tc, sendTo)
}

func (b *Bot) sourceCode(event *slack.MessageEvent) {
	_, _, err := b.slackBotAPI.PostMessage(event.Channel,
		slack.MsgOptionText("My source code is here <https://github.com/asalkeld/scrumpolice>", true),
		slack.MsgOptionAsUser(true))
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to post message to slack.")
		return
	}
}

func (b *Bot) help(event *slack.MessageEvent) {
	message := slack.Attachment{
		MarkdownIn: []string{"text"},
		Text: "- `source code`: location of my source code\n" +
			"- `help`: well, this command\n" +
			"- `tutorial`: explains how the scrum police works. Try it!\n" +
			"- `start`: starts a scrum for a team and a specific set of questions, defaults to your only team if you got only one, and only questions set if there's only one on the team you chose\n" +
			"- `restart`: restart your last done scrum, if it wasn't posted\n" +
			"- `report`: send the report to the configured channel\n" +
			"- `report-dm`: scrumpolice will direct message you the report to check\n" +
			"- `out of office`: mark current user as out of office (until `i'm back` is used)\n" +
			"- `[user] is out of office`: mark the specified user as out of office (until he or she uses `i'm back`)\n" +
			"- `i am back` or `i'm back`: mark current user as in office. MacOS smart quote can screw up with the `i'm back` command.",
	}

	_, _, err := b.slackBotAPI.PostMessage(event.Channel,
		slack.MsgOptionText("Here's a list of supported commands", true),
		slack.MsgOptionAsUser(true),
		slack.MsgOptionAttachments(message))
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to post message to slack.")
		return
	}
}

// This method sleeps to give a better feeling to the user. It should be use in a sub-routine.
func (b *Bot) tutorial(event *slack.MessageEvent) {
	b.slackBotAPI.PostMessage(event.Channel,
		slack.MsgOptionText("*Hi there* :wave: You want to know how I do things? Here :golang:es!\n"+
			"When you want to start a scrum report, just tell me `start` in a direct message :flag-dm:.\n"+
			"Then, I will ask you a couple of questions, and wait for your answers. Once you anwsered all the questions, you're done :white_check_mark:.\n"+
			"I take care of the rest! :cop:\n"+
			"When it's time :clock10:, I will post the scrum report for you and your friends in your team's channel :raised_hands:\n"+
			"All you have to do now is read the report :book: (when you have the time, I don't want to rush you :scream:)\n"+
			"That's all. Enjoy :beers:.", true),
		slack.MsgOptionAsUser(true),
	)
}

func (b *Bot) outOfOffice(event *slack.MessageEvent, userId string) {
	params := slack.MsgOptionAsUser(true)
	username := strings.TrimLeft(userId, "@")

	b.scrum.AddToOutOfOffice(username)

	if event.User == userId {
		b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText("I've marked you out of office in all your teams", true), params)
		log.WithFields(log.Fields{
			"user":   username,
			"doneBy": username,
		}).Info("User was marked out of office.")
	} else {
		b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText("I've marked @"+username+" out of office in all of your peer's teams", true), params)

		user, err := b.slackBotAPI.GetUserInfo(event.User)
		if err != nil {
			b.logSlackRelatedError(event, err, "Fail to get user information.")
			return
		}
		b.slackBotAPI.PostMessage("@"+username, slack.MsgOptionText("You've been marked out of office by @"+user.Name+".", true), params)
		log.WithFields(log.Fields{
			"user":   userId,
			"doneBy": user.Name,
		}).Info("User was marked out of office.")
	}
}

func (b *Bot) backInOffice(event *slack.MessageEvent) {
	params := slack.MsgOptionAsUser(true)
	user, err := b.slackBotAPI.GetUserInfo(event.User)
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to get user information.")
		b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText("Hmmmm, I couldn't find you. Try again!", true), params)
		return
	}
	username := user.Name

	b.scrum.RemoveFromOutOfOffice(username)
	b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText("I've marked you in office in all your teams. Welcome back!", true), params)
	log.WithFields(log.Fields{
		"user":     event.User,
		"username": event.Username,
	}).Info("User was marked in office.")
}

func (b *Bot) unrecognizedMessage(event *slack.MessageEvent) {
	log.WithFields(log.Fields{
		"text":     event.Text,
		"user":     event.User,
		"username": event.Username,
	}).Info("Received unrecognized message.")
	params := slack.MsgOptionAsUser(true)

	_, _, err := b.slackBotAPI.PostMessage(event.Channel, slack.MsgOptionText("I don't understand what you're trying to tell me, try `help`", true), params)
	if err != nil {
		b.logSlackRelatedError(event, err, "Fail to post message to slack.")
		return
	}
}

func (b *Bot) logSlackEvent(event *slack.MessageEvent, logMessage string) {
	b.logger.WithFields(log.Fields{
		"text":     event.Text,
		"user":     event.User,
		"username": event.Username,
	}).Info(logMessage)
}

func (b *Bot) logSlackRelatedError(event *slack.MessageEvent, err error, logMessage string) {
	b.logger.WithFields(log.Fields{
		"text":     event.Text,
		"user":     event.User,
		"username": event.Username,
		"error":    err,
	}).Error(logMessage)
}
