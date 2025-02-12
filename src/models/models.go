package models

import (
	"fmt"
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
		msg += fmt.Sprintf("🎲 <b>%s</b> (%d/%d players) %s\n", bg.Name, len(bg.Participants), bg.MaxPlayers, complete)
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
