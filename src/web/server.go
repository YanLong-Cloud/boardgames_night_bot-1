package web

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/web/api"
	"fmt"

	"github.com/fzerorubigd/gobgg"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

func StartServer(port int, db *database.Database, bgg *gobgg.BGG, bot *telebot.Bot, bundle *i18n.Bundle, baseUrl string) {
	var err error
	router := gin.Default()

	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*")
	//tmpl := template.Must(template.ParseFiles("templates/index.html"))
	//router.SetHTMLTemplate(tmpl)

	controller := api.NewController(router.Group("/"), db, bgg, bot, bundle, baseUrl)

	controller.InjectRoute()
	// // Serve static files
	// router.Static("/static", "./static")

	// // Route to serve the HTML page
	// router.GET("/", func(c *gin.Context) {
	// 	c.HTML(http.StatusOK, "index.html", nil)
	// })

	// // Load HTML templates

	if err = router.Run(fmt.Sprintf(":%d", port)); err != nil {
		panic(err)
	}
}
