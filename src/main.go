package main

import (
	"boardgame-night-bot/src/database"
	langpack "boardgame-night-bot/src/language"
	"boardgame-night-bot/src/models"
	"boardgame-night-bot/src/telegram"
	"boardgame-night-bot/src/web"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
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

	lp, err := langpack.BuildLanguagePack()
	if err != nil {
		log.Fatal(err)
	}

	for _, lang := range lp.Languages {
		log.Default().Printf("Loading language file: %s", lang)
		bundle.MustLoadMessageFile(fmt.Sprintf("localization/active.%s.toml", lang))
	}

	if err = godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	botToken := os.Getenv("TOKEN")
	if botToken == "" {
		log.Fatal("TOKEN is not set in .env file")
	}

	botName := os.Getenv("BOT_NAME")
	if botToken == "" {
		log.Fatal("BOT_NAME is not set in .env file")
	}

	healthCheckUrl := os.Getenv("HEALTH_CHECK_URL")
	InitHealthCheck(healthCheckUrl)

	baseUrl := os.Getenv("BASE_URL")
	portString := os.Getenv("PORT")
	port, err := strconv.Atoi(portString)
	if err != nil {
		log.Fatal("PORT is not set in .env file")
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
		Bot:            bot,
		DB:             db,
		BGG:            bgg,
		LanguageBundle: bundle,
		LanguagePack:   lp,
		BaseUrl:        baseUrl,
		BotName:        botName,
	}

	log.Println("Bot started.")

	bot.Handle("/start", telegram.Start)
	bot.Handle("/help", telegram.Start)
	bot.Handle("/create", telegram.CreateGame)
	bot.Handle("/add_game", telegram.AddGame)
	bot.Handle("/language", telegram.SetLanguage)

	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		if c.Message().ReplyTo == nil {
			return c.Respond()
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
		web.StartServer(port, db, bgg, bot, bundle, baseUrl, botName)
		log.Println("Server stopped.")
	}()
	go func() {
		log.Println("Bot started.")
		bot.Start()
		log.Println("Bot stopped.")
	}()

	select {}
}
