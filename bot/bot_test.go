package bot

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestHandleMessageIgnoreBotMessages(t *testing.T) {
	bot := Bot{}

	bot.handleMessage(&slack.MessageEvent{Msg: slack.Msg{BotID: "test"}}, false)
}

func TestCanTellIfMessageAdressedToBotByIdWithCorrectSyntax(t *testing.T) {
	bot := Bot{id: "scrumpolice",
		name: "Sylvain"}

	result := bot.adressedToMe("<@scrumpolice> hello")
	if !result {
		t.Fail()
	}
}

func TestCanTellIfMessageAdressedToBotByIdWithCorrectSyntaxButNotPrefixing(t *testing.T) {
	bot := Bot{id: "scrumpolice",
		name: "Sylvain"}

	result := bot.adressedToMe("oh, it's you <@scrumpolice> hello")
	if result {
		t.Fail()
	}
}

func TestCanTellIfMessageAdressedToBotByIdWithWrongPrefix(t *testing.T) {
	bot := Bot{id: "scrumpolice",
		name: "Sylvain"}

	result := bot.adressedToMe("@scrumpolice hello")
	if result {
		t.Fail()
	}
}

func TestCanTellIfMessageAdressedToBotByNameWithCorrectSyntax(t *testing.T) {
	bot := Bot{id: "scrumpolice",
		name: "Sylvain"}

	result := bot.adressedToMe("sylvain hello")
	if !result {
		t.Fail()
	}
}

func TestCanTellIfMessageAdressedToBotByNameWithWrongName(t *testing.T) {
	bot := Bot{id: "scrumpolice",
		name: "Sylvain"}

	result := bot.adressedToMe("Rod Stewart hello")
	if result {
		t.Fail()
	}
}

func TestCanTellIfMessageAdressedToBotByNameWithCorrectSyntaxButNotPrefixing(t *testing.T) {
	bot := Bot{id: "scrumpolice",
		name: "Sylvain"}

	result := bot.adressedToMe("oh, it's you sylvain hello")
	if result {
		t.Fail()
	}
}

func TestTrimBotNameAndIdFromMessage(t *testing.T) {
	bot := Bot{id: "scrumpolice",
		name: "Sylvain"}

	trimmedMessage := bot.trimBotNameInMessage("<@scrumpolice> sylvain it's you!!")
	if trimmedMessage != "it's you!!" {
		t.Fail()
	}
}
