package bot

import (
	"encoding/json"
	"net/http"

	"github.com/nitrictech/go-sdk/faas"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func httpResponse(ctx *faas.HttpContext, message string, status int) (*faas.HttpContext, error) {
	ctx.Response.Body = []byte(message)
	ctx.Response.Status = status
	return ctx, nil
}

func (b *Bot) EventHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error) {
	/*
		sv, err := slack.NewSecretsVerifier(ctx.Request.Headers(), b.signingSecret)
		if err != nil {
			return httpResponse(ctx, "Bad request", http.StatusBadRequest)
		}
		if _, err := sv.Write(ctx.Request.Data()); err != nil {
			return httpResponse(ctx, "Internal server error", http.StatusInternalServerError)
		}
		if err := sv.Ensure(); err != nil {
			return httpResponse(ctx, "unauthorized", http.StatusUnauthorized)
		}
	*/
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(ctx.Request.Data()), slackevents.OptionNoVerifyToken())
	if err != nil {
		return httpResponse(ctx, "Internal server error", http.StatusInternalServerError)
	}

	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal(ctx.Request.Data(), &r)
		if err != nil {
			return httpResponse(ctx, "Internal server error", http.StatusInternalServerError)
		}
		ctx.Response.Headers["Content-Type"] = []string{"text"}
		ctx.Response.Body = []byte(r.Challenge)
	}

	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			msg := slack.Msg{
				Type:      ev.Type,
				Channel:   ev.Channel,
				Text:      ev.Text,
				Timestamp: ev.TimeStamp,
				User:      ev.User,
				Username:  ev.Username,
				BotID:     ev.BotID,
			}
			if ev.BotID != "" {
				msg.BotProfile = &slack.BotProfile{
					ID:   ev.BotID,
					Name: ev.Username,
				}
			}
			b.handleMessage(&slack.MessageEvent{Msg: msg}, ev.ChannelType == "im")
		case *slackevents.AppMentionEvent:
			msg := slack.Msg{
				Type:      ev.Type,
				Channel:   ev.Channel,
				Text:      ev.Text,
				Timestamp: ev.TimeStamp,
				User:      ev.User,
				BotID:     ev.BotID,
			}
			if ev.BotID != "" {
				msg.BotProfile = &slack.BotProfile{
					ID: ev.BotID,
				}
			}
			b.handleMessage(&slack.MessageEvent{Msg: msg}, false)
		}
	}
	return next(ctx)
}
