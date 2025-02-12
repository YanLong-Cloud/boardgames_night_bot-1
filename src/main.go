package main

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"boardgame-night-bot/src/telegram"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"

	"time"

	"github.com/fzerorubigd/gobgg"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/telebot.v3"
)

var db *sql.DB

func main() {
	var err error

	if err = godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	botToken := os.Getenv("TOKEN")

	if botToken == "" {
		log.Fatal("TOKEN is not set in .env file")
	}

	db := database.NewDatabase()

	defer db.Close()

	log.Println("Database connection established.")

	db.CreateTables()

	bot, err := telebot.NewBot(telebot.Settings{
		Token:     botToken,
		ParseMode: telebot.ModeHTML,
		Poller: &telebot.LongPoller{
			Timeout:        10 * time.Second,
			AllowedUpdates: []string{"message", "callback_query"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	bgg := gobgg.NewBGGClient(gobgg.SetClient(client))

	telegram := telegram.Telegram{
		Bot: bot,
		DB:  db,
		BGG: bgg,
	}

	log.Println("Bot started.")

	bot.Handle("/create", telegram.CreateGame)

	bot.Handle("/add_game", telegram.AddGame)

	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		// check if is a reply

		if c.Message().ReplyTo == nil {
			return nil
		}

		return telegram.UpdateGameNumberOfPlayer(c)
	})

	bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		data := c.Callback().Data
		parts := strings.Split(data, "|")

		action := "$" + strings.Split(parts[0], "$")[1]

		log.Printf("User clicked on button: *%s* %d", action, len(action))
		switch action {
		case string(models.AddPlayer):
			return telegram.CallbackAddPlayer(c)
		case string(models.Cancel):
			return telegram.CallbackRemovePlayer(c)
		}

		return c.Reply("Invalid action")
	})

	bot.Start()
}
