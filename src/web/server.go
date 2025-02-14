package web

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/web/api"
	"fmt"

	"github.com/gin-gonic/gin"
)

func StartServer(port int, db *database.Database) {
	var err error
	router := gin.Default()

	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*")
	//tmpl := template.Must(template.ParseFiles("templates/index.html"))
	//router.SetHTMLTemplate(tmpl)

	controller := api.NewController(router.Group("/"), db)

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
