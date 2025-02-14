package api

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	Router *gin.RouterGroup
	DB     *database.Database
}

func NewController(router *gin.RouterGroup, db *database.Database) *Controller {
	return &Controller{
		Router: router,
		DB:     db,
	}
}

func (c *Controller) InjectRoute() {
	c.Router.GET("/events/:event_id", c.Index)
}

func (c *Controller) Index(ctx *gin.Context) {
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

	// serve an html file
	ctx.HTML(http.StatusOK, "index", gin.H{
		"Title":     event.Name,
		"Games":     event.BoardGames,
		"UpdatedAt": time.Now(),
	})

}

func P(x string) *string {
	return &x
}
