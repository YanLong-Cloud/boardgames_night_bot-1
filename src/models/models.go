package models

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

const BASE_URL = "https://3e52-2001-b07-5d31-6a42-7565-9d07-4a52-4352.ngrok-free.app"

type Event struct {
	ID         string
	ChatID     int64
	UserID     int64
	UserName   string
	MessageID  *int64
	Name       string
	BoardGames []BoardGame
}

type BoardGame struct {
	ID           int64         `json:"id"`
	Name         string        `json:"name"`
	MaxPlayers   int64         `json:"max_players"`
	Participants []Participant `json:"participants"`
	BggID        *int64        `json:"bgg_id"`
	BggName      *string       `json:"bgg_name"`
	BggUrl       *string       `json:"bgg_url"`
}

type Participant struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	UserName string `json:"user_name"`
}

// create enum with value add_player
type EventAction string

const (
	AddPlayer EventAction = "$add_player"
	Cancel    EventAction = "$cancel"
)

func (e Event) FormatMsg(localizer *i18n.Localizer, baseUrl string) (string, *telebot.ReplyMarkup) {
	btns := []telebot.InlineButton{}

	msg := "📆 <b>" + e.Name + "</b>\n\n"
	for _, bg := range e.BoardGames {
		complete := ""
		isComplete := len(bg.Participants) == int(bg.MaxPlayers)
		if isComplete {
			complete = "🚫"
		}

		link := ""
		if bg.BggUrl != nil && bg.BggName != nil {
			link = fmt.Sprintf(" - <a href='%s'>%s</a>\n", *bg.BggUrl, *bg.BggName)
		}

		msg += fmt.Sprintf("🎲 <b>%s [%s]</b> (%d/%d players) %s\n", link, bg.Name, len(bg.Participants), bg.MaxPlayers, complete)
		for _, p := range bg.Participants {
			msg += " - " + p.UserName + "\n"
		}
		msg += "\n"

		joinT := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID: "Join",
			},
			TemplateData: map[string]string{
				"Name": bg.Name,
			},
		})
		btn := telebot.InlineButton{
			Text:   joinT,
			Unique: string(AddPlayer),
			Data:   fmt.Sprintf("%s|%d", e.ID, bg.ID),
		}

		btns = append(btns, btn)

	}

	msg += localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "UpdatedAt",
		},
		TemplateData: map[string]string{
			"Time": time.Now().Format("2006-01-02 15:04:05"),
		},
	})

	btn := telebot.InlineButton{
		Text:   "Not coming",
		Unique: string(Cancel),
		Data:   e.ID,
	}

	btns = append(btns, btn)

	btn2 := telebot.InlineButton{
		Text: "Web",
		WebApp: &telebot.WebApp{
			URL: fmt.Sprintf("%s/events/%s", baseUrl, e.ID),
		},
	}

	btns = append(btns, btn2)

	markup := &telebot.ReplyMarkup{}
	markup.InlineKeyboard = [][]telebot.InlineButton{}
	for _, btn := range btns {
		markup.InlineKeyboard = append(markup.InlineKeyboard, []telebot.InlineButton{btn})
	}

	return msg, markup
}

func ExtractBoardGameID(inputURL string) (int64, bool) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return 0, false
	}

	// Ensure the scheme is HTTPS and the host is correct
	if parsedURL.Scheme != "https" || parsedURL.Host != "boardgamegeek.com" {
		return 0, false
	}

	// Define regex to extract the ID
	pattern := `^/boardgame/(\d+)/[a-zA-Z0-9-]+$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(parsedURL.Path)

	if len(matches) > 1 {
		id, err := strconv.ParseInt(matches[1], 10, 64)
		return id, err == nil
	}
	return 0, false
}

func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}
