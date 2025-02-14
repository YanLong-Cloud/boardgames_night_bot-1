package main

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"boardgame-night-bot/src/telegram"
	"boardgame-night-bot/src/web"
	"log"
	"net/http"
	"os"
	"strings"

	"time"

	"github.com/BurntSushi/toml"
	"github.com/fzerorubigd/gobgg"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/robfig/cron/v3"
	"golang.org/x/text/language"
	"gopkg.in/telebot.v3"
)

func callEndpoint(url string) func() {
	return func() {
		resp, err := http.Get(url)
		if err != nil {
			log.Println("Error calling endpoint:", err)
			return
		}

		defer resp.Body.Close()
		log.Println("Endpoint called successfully at", time.Now())
	}
}

func InitHealthCheck(url string) {
	if url == "" {
		log.Println("HEALTH_CHECK_URL is not set in .env file")
		return
	}

	defer callEndpoint(url)()

	c := cron.New()
	_, err := c.AddFunc("@hourly", callEndpoint(url))
	if err != nil {
		log.Println("Error scheduling cron job:", err)
		return
	}

	c.Start()
	log.Println("Cron job started...")
}

func main() {
	var err error

	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	bundle.MustLoadMessageFile("localization/active.en.toml")
	bundle.MustLoadMessageFile("localization/active.it.toml")

	if err = godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	botToken := os.Getenv("TOKEN")
	if botToken == "" {
		log.Fatal("TOKEN is not set in .env file")
	}

	healthCheckUrl := os.Getenv("HEALTH_CHECK_URL")
	InitHealthCheck(healthCheckUrl)

	baseUrl := os.Getenv("BASE_URL")

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
		Bot:            bot,
		DB:             db,
		BGG:            bgg,
		LanguageBundle: bundle,
		BaseUrl:        baseUrl,
	}

	log.Println("Bot started.")

	bot.Handle("/start", telegram.Start)
	bot.Handle("/help", telegram.Start)
	bot.Handle("/create", telegram.CreateGame)
	bot.Handle("/add_game", telegram.AddGame)
	bot.Handle("/language", telegram.SetLanguage)

	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		if c.Message().ReplyTo == nil {
			return nil
		}

		return telegram.UpdateGameDispatcher(c)
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

	go func() {
		log.Println("Server started.")
		web.StartServer(8080, db)
		log.Println("Server stopped.")
	}()
	go func() {
		log.Println("Bot started.")
		bot.Start()
		log.Println("Bot stopped.")
	}()

	select {}
}
