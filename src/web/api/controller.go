package api

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/fzerorubigd/gobgg"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

type Controller struct {
	Router         *gin.RouterGroup
	DB             *database.Database
	BGG            *gobgg.BGG
	Bot            *telebot.Bot
	LanguageBundle *i18n.Bundle
	BaseUrl        string
	BotName        string
}

func NewController(router *gin.RouterGroup, db *database.Database, bgg *gobgg.BGG, bot *telebot.Bot, LanguageBundle *i18n.Bundle, baseUrl, botName string) *Controller {
	return &Controller{
		Router:         router,
		DB:             db,
		BGG:            bgg,
		Bot:            bot,
		LanguageBundle: LanguageBundle,
		BaseUrl:        baseUrl,
		BotName:        botName,
	}
}

func (t Controller) Localizer(chatID int64) *i18n.Localizer {
	return i18n.NewLocalizer(t.LanguageBundle, t.DB.GetPreferredLanguage(chatID), "en")
}

func (c *Controller) InjectRoute() {
	c.Router.GET("/", c.Index)
	c.Router.GET("/events/:event_id", c.Event)
	c.Router.POST("/events/:event_id/add-game", c.AddGame)
	c.Router.POST("/events/:event_id/join", c.AddPlayer)
}

func (c *Controller) Index(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "index", nil)
}

func (c *Controller) Event(ctx *gin.Context) {
	var err error
	eventID := ctx.Param("event_id")

	if !models.IsValidUUID(eventID) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	var event *models.Event

	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("Failed to load game:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	localizer := c.Localizer(event.ChatID)
	timeT := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "WebUpdatedAt",
		},
		TemplateData: map[string]string{
			"Time": time.Now().Format("2006-01-02 15:04:05"),
		},
	})

	// serve an html file
	ctx.HTML(http.StatusOK, "event", gin.H{
		"Id":             event.ID,
		"Title":          event.Name,
		"Games":          event.BoardGames,
		"UpdatedAt":      timeT,
		"NoParticipants": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebNoParticipants"}),
		"Players":        localizer.MustLocalizeMessage(&i18n.Message{ID: "WebPlayers"}),
		"Join":           localizer.MustLocalizeMessage(&i18n.Message{ID: "WebJoin"}),
		"AddGame":        localizer.MustLocalizeMessage(&i18n.Message{ID: "WebAddGame"}),
		"Welcome":        localizer.MustLocalizeMessage(&i18n.Message{ID: "WebWelcome"}),
		"AddNewGame":     localizer.MustLocalizeMessage(&i18n.Message{ID: "WebAddNewGame"}),
		"GameName":       localizer.MustLocalizeMessage(&i18n.Message{ID: "WebGameName"}),
		"MaxPlayers":     localizer.MustLocalizeMessage(&i18n.Message{ID: "WebMaxPlayers"}),
	})

}

func (c *Controller) AddGame(ctx *gin.Context) {
	var err error
	eventID := ctx.Param("event_id")

	if !models.IsValidUUID(eventID) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	var event *models.Event

	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("Failed to load game:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	var bg models.AddGameRequest
	if err = ctx.ShouldBind(&bg); err != nil {
		log.Println("Failed to bind form:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	if event.Locked && event.UserID != bg.UserID {
		log.Println("Event is locked")
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Unable to add game to locked event"})
		return
	}

	if bg.MaxPlayers == nil {
		defaultMax := 5
		bg.MaxPlayers = &defaultMax
	}

	bgCtx := context.Background()

	var bgID *int64
	var bgName, bgUrl *string
	if bg.BggUrl != nil && *bg.BggUrl != "" {
		var valid bool
		var id int64
		if id, valid = models.ExtractBoardGameID(*bg.BggUrl); !valid {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid bgg url"})
			return
		}

		var bgMaxPlayers *int

		if bgMaxPlayers, bgName, bgUrl, err = models.ExtractGameInfo(bgCtx, c.BGG, id, bg.Name); err != nil {
			log.Printf("Failed to get game %d: %v", id, err)
		} else {
			bgID = &id
			bg.MaxPlayers = bgMaxPlayers
		}
	} else {
		log.Printf("Searching for game %s", bg.Name)
		var results []gobgg.SearchResult

		if results, err = c.BGG.Search(bgCtx, bg.Name); err != nil {
			log.Printf("Failed to search game %s: %v", bg.Name, err)
		}

		if len(results) == 0 {
			log.Printf("Game %s not found", bg.Name)
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

			log.Printf("Game %s id %d found: %s", bg.Name, *bgID, *bgUrl)

			var things []gobgg.ThingResult

			if things, err = c.BGG.GetThings(bgCtx, gobgg.GetThingIDs(*bgID)); err != nil {
				log.Printf("Failed to get game %s: %v", bg.Name, err)
			}

			if len(things) > 0 {
				log.Printf("Game details of %s found", bg.Name)
				if things[0].MaxPlayers > 0 {
					bg.MaxPlayers = &things[0].MaxPlayers
				}

				if things[0].Name != "" {
					bgName = &things[0].Name
				} else {
					bgName = &bg.Name
				}
			}
		}
	}

	log.Printf("Inserting %s in the db", bg.Name)

	if _, err = c.DB.InsertBoardGame(event.ID, bg.Name, *bg.MaxPlayers, bgID, bgName, bgUrl); err != nil {
		log.Println("Failed to insert board game:", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert board game"})
		return
	}

	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("Failed to load game:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	if event.MessageID == nil {
		log.Println("Event message id is nil")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid message ID"})
		return
	}

	body, markup := event.FormatMsg(c.Localizer(event.ChatID), c.BaseUrl, c.BotName)

	_, err = c.Bot.Edit(&telebot.Message{
		ID: int(*event.MessageID),
		Chat: &telebot.Chat{
			ID: event.ChatID,
		},
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			log.Println("Failed because unchanged", err)
		}

	}

	var game *models.BoardGame
	for _, g := range event.BoardGames {
		if g.Name == bg.Name {
			game = &g
			break
		}
	}

	// ctx.JSON(http.StatusOK, gin.H{"message": "Board game added successfully"})
	ctx.HTML(http.StatusOK, "game_info", gin.H{
		"Id":    event.ID,
		"Title": event.Name,
		"Game":  game,
	})
}

func (c *Controller) AddPlayer(ctx *gin.Context) {
	var err error
	var event *models.Event
	eventID := ctx.Param("event_id")

	if !models.IsValidUUID(eventID) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	var addPlayer models.AddPlayerRequest
	if err = ctx.ShouldBindJSON(&addPlayer); err != nil {
		log.Println("Failed to bind form:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	if _, err = c.DB.InsertParticipant(eventID, addPlayer.GameID, addPlayer.UserID, addPlayer.UserName); err != nil {
		log.Println("Failed to add user to participants table:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("Failed to load game:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	if event.MessageID == nil {
		log.Println("Event message id is nil")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid message ID"})
		return
	}

	body, markup := event.FormatMsg(c.Localizer(event.ChatID), c.BaseUrl, c.BotName)

	_, err = c.Bot.Edit(&telebot.Message{
		ID: int(*event.MessageID),
		Chat: &telebot.Chat{
			ID: event.ChatID,
		},
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			log.Println("Failed because unchanged", err)
		}

	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Player added."})
}

func P(x string) *string {
	return &x
}
