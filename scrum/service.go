package scrum

import (
	"fmt"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/nitrictech/go-sdk/api/documents"
	"github.com/nitrictech/go-sdk/resources"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

var SlackParams = func() slack.MsgOption {
	p := slack.NewPostMessageParameters()
	p.AsUser = true
	p.LinkNames = 1
	return slack.MsgOptionPostMessageParameters(p)
}()

type Service interface {
	GetTeamByName(team string) (*TeamConfig, error)
	GetTeamForUser(username string) *TeamConfig
	GetAllTeamMembers(team string) ([]*UserState, error)

	GetUserState(username string) *UserState
	SaveUserState(us *UserState) error

	AddToOutOfOffice(username string)
	RemoveFromOutOfOffice(username string)

	SendReportForTeam(tc *TeamConfig, sendTo string)
}

type service struct {
	configurationProvider ConfigurationProvider
	slackBotAPI           *slack.Client
}

var (
	userStateCol documents.CollectionRef
)

func (mod *service) postMessageToSlack(channel string, message string, params ...slack.MsgOption) {
	_, _, err := mod.slackBotAPI.PostMessage(channel, append(params, slack.MsgOptionText(message, true))...)
	if err != nil {
		log.WithFields(log.Fields{
			"channel": channel,
			"error":   err,
		}).Warn("Error while posting message to slack")
	}
}

func (mod *service) SendReportForTeam(tc *TeamConfig, sendTo string) {
	attachments := []slack.Attachment{}
	didNotDoReport := []string{}
	outOfOffice := []string{}

	members, err := mod.GetAllTeamMembers(tc.Name)
	if err != nil {
		fmt.Println(err)
	}

	for _, member := range members {
		if member.OutOfOffice {
			outOfOffice = append(outOfOffice, member.User)
		} else if len(member.Answers) == 0 {
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
				message += q + "\n" + member.Answers[q]

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

	if tc.SplitReport {
		mod.postMessageToSlack(sendTo, ":parrotcop: Alrighty! Here's the scrum report for today!", SlackParams)
		for i := 0; i < len(attachments); i++ {
			mod.postMessageToSlack(sendTo, "*Scrum by:*", SlackParams, slack.MsgOptionAttachments(attachments[i]))
		}
	} else {
		mod.postMessageToSlack(sendTo, ":parrotcop: Alrighty! Here's the scrum report for today!", SlackParams, slack.MsgOptionAttachments(attachments...))
	}

	if len(didNotDoReport) > 0 {
		mod.postMessageToSlack(sendTo, fmt.Sprintln("And lastly we should take a little time to shame", didNotDoReport), SlackParams)
	}

	log.WithFields(log.Fields{
		"team":    tc.Name,
		"channel": tc.Channel,
	}).Info("Sent scrum report.")
}

func NewService(configurationProvider ConfigurationProvider, slackBotAPI *slack.Client) Service {
	mod := &service{
		configurationProvider: configurationProvider,
		slackBotAPI:           slackBotAPI,
	}

	var err error
	userStateCol, err = resources.NewCollection("userState", resources.CollectionWriting, resources.CollectionReading, resources.CollectionDeleting)
	if err != nil {
		panic(err)
	}

	configurationProvider.OnChange(func(cfg *Config) {
		log.Println("Configuration File Changed refreshing state")
		mod.SetReminders(cfg)
	})

	return mod
}

func (mod *service) SetReminders(config *Config) {
	log.Info("Refreshing teams.")
}

func (m *service) GetTeamForUser(username string) *TeamConfig {
	tcs, err := m.GetAllTeams()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	for _, tc := range tcs {
		for _, member := range tc.Members {
			if username == member {
				return tc
			}
		}
	}
	return nil
}

func (m *service) GetAllTeamMembers(team string) ([]*UserState, error) {
	tc, err := m.GetTeamByName(team)
	if err != nil {
		return nil, err
	}

	all := []*UserState{}
	for _, member := range tc.Members {
		all = append(all, m.GetUserState(member))
	}

	return all, nil
}

func (m *service) GetAllTeams() ([]*TeamConfig, error) {
	query := teamCol.Query()
	results, err := query.Fetch()
	if err != nil {
		return nil, err
	}

	all := []*TeamConfig{}
	for _, doc := range results.Documents {
		tc := &TeamConfig{}
		err = grpcSafeDecode(doc.Content(), tc)
		if err != nil {
			return nil, err
		}
		all = append(all, tc)
	}
	return all, nil
}

func (m *service) GetTeamByName(teamName string) (*TeamConfig, error) {
	doc, err := teamCol.Doc(teamName).Get()
	if err != nil {
		return nil, err
	}
	tc := &TeamConfig{}
	err = grpcSafeDecode(doc.Content(), tc)
	return tc, err
}

func (m *service) GetUserState(username string) *UserState {
	doc, err := userStateCol.Doc(username).Get()
	us := &UserState{}
	if err != nil {
		fmt.Println(err)

		us.User = username
		us.Answers = map[string]string{}
		return us
	}
	err = grpcSafeDecode(doc.Content(), us)
	if err != nil {
		fmt.Println(err)
	}
	return us
}

func (m *service) GetQuestionsForTeam(team string) []string {
	tc, err := m.GetTeamByName(team)
	if err != nil {
		return []string{}
	}
	return tc.Questions
}

func (m *service) SaveUserState(us *UserState) error {
	userStateMap := map[string]interface{}{}
	err := grpcSafeDecode(us, &userStateMap)
	if err != nil {
		return err
	}
	userStateMap = grpcSafeFix(userStateMap)

	return userStateCol.Doc(us.User).Set(userStateMap)
}

func (m *service) AddToOutOfOffice(username string) {
	us := m.GetUserState(username)
	us.OutOfOffice = true

	if err := m.SaveUserState(us); err != nil {
		fmt.Println(err)
	}
}

func (m *service) RemoveFromOutOfOffice(username string) {
	us := m.GetUserState(username)
	us.OutOfOffice = false

	if err := m.SaveUserState(us); err != nil {
		fmt.Println(err)
	}
}
