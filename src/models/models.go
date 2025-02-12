package models

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"gopkg.in/telebot.v3"
)

type Event struct {
	ID         int64
	ChatID     int64
	UserID     int64
	UserName   string
	MessageID  *int64
	Name       string
	BoardGames []BoardGame
}

type BoardGame struct {
	ID           int64
	Name         string
	MaxPlayers   int64
	Participants []Participant
	BggID        *int64
	BggName      *string
	BggUrl       *string
}

type Participant struct {
	ID       int64
	UserID   int64
	UserName string
}

// create enum with value add_player
type EventAction string

const (
	AddPlayer EventAction = "$add_player"
	Cancel    EventAction = "$cancel"
)

func (e Event) FormatMsg() (string, *telebot.ReplyMarkup) {
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

		btn := telebot.InlineButton{
			Text:   "Join " + bg.Name,
			Unique: string(AddPlayer),
			Data:   fmt.Sprintf("%d|%d", e.ID, bg.ID),
		}

		btns = append(btns, btn)

	}
	msg += fmt.Sprintf("<i>update at %s</i>\n/add_game to add more game", time.Now().Format("2006-01-02 15:04:05"))

	btn := telebot.InlineButton{
		Text:   "Not coming",
		Unique: string(Cancel),
		Data:   fmt.Sprintf("%d", e.ID),
	}

	btns = append(btns, btn)

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
