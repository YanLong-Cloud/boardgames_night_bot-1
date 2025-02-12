package telegram

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/telebot.v3"
)

type Telegram struct {
	Bot *telebot.Bot
	DB  *database.Database
}

func (t Telegram) CreateGame(c telebot.Context) error {
	var err error
	args := c.Args()
	if len(args) < 1 {
		return c.Reply("Usage: /create event_name")
	}
	eventName := strings.Join(args[0:], " ")
	userID := c.Sender().ID
	userName := c.Sender().Username
	chatID := c.Chat().ID
	var eventID int64
	log.Printf("Creating event: %s by user: %s (%d) in chat: %d", eventName, userName, userID, chatID)

	if eventID, err = t.DB.InsertEvent(chatID, userID, userName, eventName, nil); err != nil {
		log.Println("Failed to create event:", err)
		return c.Reply("Failed to create event: " + err.Error())
	}

	body := fmt.Sprintf("🎲 '%s'\nNo game added yet please /add_game to add games.", eventName)

	responseMsg, err := t.Bot.Reply(c.Message(), body)
	if err != nil {
		log.Println("Failed to create event:", err)
		return c.Reply("Failed to create event: " + err.Error())
	}

	if t.DB.UpdateEventMessageID(eventID, int64(responseMsg.ID)); err != nil {
		log.Println("Failed to create event:", err)
		return c.Reply("Failed to create event: " + err.Error())
	}

	return err
}

func (t Telegram) AddGame(c telebot.Context) error {
	var err error
	log.Println("User requested to add a game.")

	args := c.Args()
	if len(args) < 1 {
		return c.Reply("Usage: /add_game game_name")
	}

	chatID := c.Chat().ID
	userID := c.Sender().ID
	userName := c.Sender().Username
	gameName := strings.Join(args[0:], " ")
	maxPlayers := 5
	log.Printf("Adding game: %s with max players: %d", gameName, maxPlayers)

	var event *models.Event
	var boardGameID int64

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		return c.Reply("Event not found in the db " + err.Error())
	}

	if boardGameID, err = t.DB.InsertBoardGame(event.ID, gameName, maxPlayers); err != nil {
		log.Println("Failed to add game:", err)
		return c.Reply("Failed to add game: " + err.Error())
	}

	if _, err = t.DB.InsertParticipant(event.ID, boardGameID, userID, userName); err != nil {
		log.Println("Failed to add user to participants table:", err)
		return c.Reply("Failed to add user to participants table: " + err.Error())
	}

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		return c.Reply("Event not found in the db " + err.Error())
	}

	body, markup := event.FormatMsg()

	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup)
	if err != nil {
		log.Println("Failed to edit message")
		return c.Reply("Failed to edit message event: " + err.Error())
	}

	responseMsg, err := t.Bot.Reply(c.Message(), fmt.Sprintf("Game <b>%s</b> added! [0/%d] players.\nReply to this message with the max number of player to update (default 5)\nClick button to join.", gameName, maxPlayers))
	if err != nil {
		log.Println("Failed to create event:", err)
		return c.Reply("Failed to create event: " + err.Error())
	}

	if err = t.DB.UpdateBoardGameMessageID(boardGameID, int64(responseMsg.ID)); err != nil {
		log.Println("Failed to update boardgame id:", err)
		return c.Reply("Failed to update boardgame: " + err.Error())
	}

	return c.Respond()
}

func (t Telegram) UpdateGameNumberOfPlayer(c telebot.Context) error {
	var err error
	chatID := c.Chat().ID
	messageID := c.Message().ReplyTo.ID
	maxPlayerS := c.Text()

	maxPlayers, err2 := strconv.ParseInt(maxPlayerS, 10, 64)
	if err2 != nil {
		return c.Reply("Invalid number of players")
	}

	log.Printf("Updating game message id: %d with number of players: %d", messageID, maxPlayers)

	if err = t.DB.UpdateBoardGamePlayerNumber(int64(messageID), int(maxPlayers)); err != nil {
		if errors.Is(err, database.ErrNoRows) {
			return c.Reply("Game not found. You are trying to update the number of players of a game that does not exist. You are probably commenting on the wrong message.")
		}

		log.Println("Failed to update game:", err)
		return c.Reply("Failed to update game: " + err.Error())
	}

	var event *models.Event

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		return c.Reply("Event not found in the db " + err.Error())
	}

	body, markup := event.FormatMsg()

	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup)
	if err != nil {
		log.Println("Failed to edit message")
		return c.Reply("Failed to edit message event: " + err.Error())
	}

	return c.Reply("Game updated")
}

func (t Telegram) CallbackAddPlayer(c telebot.Context) error {
	var event *models.Event
	var err error

	data := c.Callback().Data
	parts := strings.Split(data, "|")
	if len(parts) != 3 {
		log.Println("Invalid data:", data)
		return c.Reply("Invalid data: " + data)
	}

	eventID, err1 := strconv.ParseInt(parts[1], 10, 64)
	boardGameID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		log.Println("Invalid parsed id:", data)
		return c.Reply("Invalid parsed id: " + data)
	}

	chatID := c.Chat().ID
	userID := c.Sender().ID
	userName := c.Sender().Username
	log.Printf("User %s (%d) clicked to join a game.", userName, userID)

	if _, err = t.DB.InsertParticipant(eventID, boardGameID, userID, userName); err != nil {
		log.Println("Failed to add user to participants table:", err)
		return c.Reply("Failed to add user to participants table: " + err.Error())
	}

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		return c.Reply("Event not found in the db " + err.Error())
	}

	body, markup := event.FormatMsg()
	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup)
	if err != nil {
		log.Println("Failed to edit message")
		return c.Reply("Failed to edit message event: " + err.Error())
	}

	// Implement logic to add user to participants table here
	return c.Respond()
}

func (t Telegram) CallbackRemovePlayer(c telebot.Context) error {
	var event *models.Event
	var err error

	data := c.Callback().Data
	parts := strings.Split(data, "|")
	if len(parts) != 2 {
		log.Println("Invalid data:", data)
		return c.Reply("Invalid data: " + data)
	}

	eventID, err1 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil {
		log.Println("Invalid parsed id:", data)
		return c.Reply("Invalid parsed id: " + data)
	}

	chatID := c.Chat().ID
	userID := c.Sender().ID
	userName := c.Sender().Username
	log.Printf("User %s (%d) clicked to exit a game.", userName, userID)

	if err = t.DB.RemoveParticipant(eventID, userID); err != nil {
		log.Println("Failed to remove user to participants table:", err)
		return c.Reply("Failed to remove user to participants table: " + err.Error())
	}

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		return c.Reply("Event not found in the db " + err.Error())
	}

	body, markup := event.FormatMsg()
	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup)
	if err != nil {
		log.Println("Failed to edit message")
		return c.Reply("Failed to edit message event: " + err.Error())
	}

	// Implement logic to add user to participants table here
	return c.Respond()
}
