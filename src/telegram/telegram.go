package telegram

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/fzerorubigd/gobgg"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/telebot.v3"
)

type Telegram struct {
	Bot *telebot.Bot
	DB  *database.Database
	BGG *gobgg.BGG
}

const MessageUnchangedErrorMessage = "specified new message content and reply markup are exactly the same as a current content and reply markup of the message"

func DefineUsername(user *telebot.User) string {
	if user.Username != "" {
		return user.Username
	}

	username := fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	if username != " " {
		return username
	}

	return fmt.Sprintf("user_%d", user.ID)
}

func (t Telegram) Start(c telebot.Context) error {
	return c.Send(`Welcome to Boardgame Night Bot! 🎲
	We are here to help you organize your boardgame night.
	Use /create [event name] to create a new event and /add_game [game name] to add games to the event.
	Click on the buttons to join or leave a game.
	Have fun! 🎉`)
}

func (t Telegram) CreateGame(c telebot.Context) error {
	var err error
	args := c.Args()
	if len(args) < 1 {
		return c.Reply("Usage: /create event_name")
	}
	eventName := strings.Join(args[0:], " ")
	userID := c.Sender().ID
	userName := DefineUsername(c.Sender())
	chatID := c.Chat().ID
	var eventID int64
	log.Printf("Creating event: %s by user: %s (%d) in chat: %d", eventName, userName, userID, chatID)

	if eventID, err = t.DB.InsertEvent(chatID, userID, userName, eventName, nil); err != nil {
		log.Println("Failed to create event:", err)
		return c.Reply("Failed to create event: " + err.Error())
	}

	body := fmt.Sprintf("📆 <b>%s</b>\nNo game added yet please /add_game to add games.", eventName)

	responseMsg, err := t.Bot.Reply(c.Message(), body, telebot.NoPreview)
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

func (t Telegram) extractGameInfo(ctx context.Context, id int64, gameName string) (*int, *string, *string, error) {
	var err error
	var bgUrl, bgName *string
	var maxPlayers *int
	url := fmt.Sprintf("https://boardgamegeek.com/boardgame/%d", id)
	bgUrl = &url

	var things []gobgg.ThingResult

	if things, err = t.BGG.GetThings(ctx, gobgg.GetThingIDs(id)); err != nil {
		log.Printf("Failed to get game %d: %v", id, err)
		return nil, nil, nil, err
	}

	if len(things) > 0 {
		maxPlayers = &things[0].MaxPlayers
		if things[0].Name != "" {
			bgName = &things[0].Name
		} else {
			bgName = &gameName
		}
	}

	return maxPlayers, bgName, bgUrl, nil
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
	userName := DefineUsername(c.Sender())
	gameName := strings.Join(args[0:], " ")
	maxPlayers := 5
	log.Printf("Adding game: %s with max players: %d", gameName, maxPlayers)

	var event *models.Event
	var boardGameID int64

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		return c.Reply("Event not found in the db " + err.Error())
	}

	ctx := context.Background()
	var results []gobgg.SearchResult
	var bgUrl, bgName *string
	var bgID *int64

	if results, err = t.BGG.Search(ctx, gameName); err != nil {
		log.Printf("Failed to search game %s: %v", gameName, err)
	}

	if len(results) == 0 {
		log.Printf("Game %s not found", gameName)
	} else {
		sort.Slice(results, func(i, j int) bool {
			return results[i].ID < results[j].ID
		})

		url := fmt.Sprintf("https://boardgamegeek.com/boardgame/%d", results[0].ID)
		bgUrl = &url
		if results[0].Name != "" {
			bgName = &results[0].Name
		}

		bgID = &results[0].ID

		log.Printf("Game %s id %d found: %s", gameName, *bgID, *bgUrl)

		var things []gobgg.ThingResult

		if things, err = t.BGG.GetThings(ctx, gobgg.GetThingIDs(*bgID)); err != nil {
			log.Printf("Failed to get game %s: %v", gameName, err)
		}

		if len(things) > 0 {
			maxPlayers = things[0].MaxPlayers
			if things[0].Name != "" {
				bgName = &things[0].Name
			} else {
				bgName = &gameName
			}
		}
	}

	if boardGameID, err = t.DB.InsertBoardGame(event.ID, gameName, maxPlayers, bgID, bgName, bgUrl); err != nil {
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
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), MessageUnchangedErrorMessage) {
			return c.Respond()
		}
		return c.Reply("Failed to edit message event: " + err.Error())
	}

	link := ""
	if bgUrl != nil && bgName != nil {
		link = fmt.Sprintf(", <a href='%s'>%s</a>", *bgUrl, *bgName)
	}

	message := fmt.Sprintf("Game <b>%s</b>%s added! (1/%d players).\nReply to this message with the max number of player to update (default %d)\nYou can also send me the https://boardgamegeek.com/ link to update the game info.\nClick button to join.", gameName, link, maxPlayers, maxPlayers)

	responseMsg, err := t.Bot.Reply(
		c.Message(),
		message,
		telebot.NoPreview,
	)
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

func (t Telegram) UpdateGameDispatcher(c telebot.Context) error {
	if c.Message().ReplyTo == nil {
		return nil
	}

	if strings.HasPrefix(c.Text(), "https://boardgamegeek.com/boardgame/") {
		return t.UpdateGameBGGInfo(c)
	}

	return t.UpdateGameNumberOfPlayer(c)
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
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), MessageUnchangedErrorMessage) {
			return c.Respond()
		}

		return c.Reply("Failed to edit message event: " + err.Error())
	}

	return c.Reply("Game updated")
}

func (t Telegram) UpdateGameBGGInfo(c telebot.Context) error {
	var err error
	chatID := c.Chat().ID
	messageID := c.Message().ReplyTo.ID
	bggURL := strings.Trim(c.Text(), " ")

	var valid bool
	var id int64
	if id, valid = models.ExtractBoardGameID(bggURL); !valid {
		return c.Reply("Invalid BGG URL")
	}

	ctx := context.Background()
	var maxPlayers *int
	var bgName, bgUrl *string

	if maxPlayers, bgName, bgUrl, err = t.extractGameInfo(ctx, id, "old name"); err != nil {
		return c.Reply("Failed to get game info: " + err.Error())
	}

	log.Printf("Updating game message id: %d with number of players: %d", messageID, maxPlayers)

	if err = t.DB.UpdateBoardGameBGGInfo(int64(messageID), *maxPlayers, &id, bgName, bgUrl); err != nil {
		if errors.Is(err, database.ErrNoRows) {
			return c.Reply("Game not found. You are trying to update the information of a game that does not exist. You are probably commenting on the wrong message.")
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
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), MessageUnchangedErrorMessage) {
			return c.Respond()
		}

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
	userName := DefineUsername(c.Sender())
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
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), MessageUnchangedErrorMessage) {
			return c.Respond()
		}
		return c.Reply("Failed to edit message event: " + err.Error())
	}

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
	userName := DefineUsername(c.Sender())
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
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), MessageUnchangedErrorMessage) {
			return c.Respond()
		}
		return c.Reply("Failed to edit message event: " + err.Error())
	}

	return c.Respond()
}
