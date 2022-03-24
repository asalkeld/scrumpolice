package scrum

import (
	"errors"
	"fmt"
	"strings"

	"github.com/asalkeld/scrumpolice/common"
	"github.com/nitrictech/go-sdk/api/documents"
	"github.com/nitrictech/go-sdk/faas"
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
	GetAllTeams() ([]*TeamConfig, error)
	GetTeamByName(team string) (*TeamConfig, error)
	GetTeamForUser(username string) *TeamConfig
	GetAllTeamMembers(team string) ([]*UserState, error)
	SaveTeamConfig(tc *TeamConfig) error

	GetUserState(username string) *UserState
	SaveUserState(us *UserState) error

	AddToOutOfOffice(username string)
	RemoveFromOutOfOffice(username string)

	SendReportForTeam(tc *TeamConfig, sendTo string) error
	RunReports() error
}

type service struct {
	configurationProvider ConfigurationProvider
	slackBotAPI           *slack.Client
}

var (
	userStateCol documents.CollectionRef
)

func NewService(configurationProvider ConfigurationProvider, slackBotAPI *slack.Client) (Service, error) {
	mod := &service{
		configurationProvider: configurationProvider,
		slackBotAPI:           slackBotAPI,
	}

	var err error
	userStateCol, err = resources.NewCollection("userState", resources.CollectionWriting, resources.CollectionReading, resources.CollectionDeleting)
	if err != nil {
		return nil, err
	}

	err = resources.NewSchedule("sendReport", "30 minutes", func(ec *faas.EventContext, next faas.EventHandler) (*faas.EventContext, error) {
		fmt.Println("Got scheduled event")

		err := mod.RunReports()
		if err != nil {
			fmt.Println("RunReports returned an error ", err)
		}
		return next(ec)
	})
	if err != nil {
		return nil, err
	}

	return mod, nil
}

func (mod *service) postMessageToSlack(channel string, message string, params ...slack.MsgOption) {
	_, _, err := mod.slackBotAPI.PostMessage(channel, append(params, slack.MsgOptionText(message, true))...)
	if err != nil {
		log.WithFields(log.Fields{
			"channel": channel,
			"error":   err,
		}).Warn("Error while posting message to slack")
	}
}

func (mod *service) SendReportForTeam(tc *TeamConfig, sendTo string) error {
	today, err := common.ToDay(tc.Timezone)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(sendTo, "@") && today == tc.LastSendDate {
		// already been sent
		return nil
	}

	members, err := mod.GetAllTeamMembers(tc.Name)
	if err != nil {
		return err
	}

	attachments, didNotDoReport := tc.GenerateReport(today, members)

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

	if !strings.HasPrefix(sendTo, "@") {
		tc.LastSendDate = today
		mod.SaveTeamConfig(tc)
	}

	log.WithFields(log.Fields{
		"team":    tc.Name,
		"channel": tc.Channel,
	}).Info("Sent scrum report.")

	return nil
}

func (ss *service) RunReports() error {
	errs := []error{}
	for _, tc := range ss.configurationProvider.Config().Teams {
		ready, err := tc.ReadyToSendReport()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		fmt.Printf("team %s report ready:%v\n", tc.Name, ready)
		if ready {
			err = ss.SendReportForTeam(&tc, tc.Channel)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		msg := ""
		for _, e := range errs {
			msg += e.Error() + "\n"
		}
		return errors.New(msg)
	}
	return nil
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
		err = decodeWithJsonTags(doc.Content(), tc)
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
	err = decodeWithJsonTags(doc.Content(), tc)
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
	err = decodeWithJsonTags(doc.Content(), us)
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

func (m *service) SaveTeamConfig(tc *TeamConfig) error {
	teamMap := map[string]interface{}{}
	err := decodeWithJsonTags(tc, &teamMap)
	if err != nil {
		return err
	}

	return teamCol.Doc(tc.Name).Set(teamMap)
}

func (m *service) SaveUserState(us *UserState) error {
	userStateMap := map[string]interface{}{}
	err := decodeWithJsonTags(us, &userStateMap)
	if err != nil {
		return err
	}

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
