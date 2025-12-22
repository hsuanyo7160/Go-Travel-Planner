package main

import (
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// 載入 .env 檔案
	godotenv.Load()

	// 連線 MongoDB
	initMongo()

	// 設定 Gin
	r := gin.Default()

	// CORS 設定 - 允許前端跨域請求
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8080", "*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 靜態檔案服務 - 使用原本的 /static 資料夾
	r.Static("/web", "../static")
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/web/")
	})

	// API 路由
	api := r.Group("/api")
	{
		// 行程相關
		api.GET("/trips", getTrips)
		api.GET("/trips/:id", getTrip)
		api.POST("/trips", createTrip)
		api.PUT("/trips/:id", updateTrip)
		api.DELETE("/trips/:id", deleteTrip)

		// Gemini 相關
		api.POST("/gemini", callGemini) // 一般問答
		api.POST("/gemini/save", saveGeminiToFile)
		api.GET("/gemini/response", getGeminiResponse)

		api.POST("/gemini/chat", chatWithGemini) // 對話模式

		// 健康檢查
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "ok",
				"time":   time.Now(),
			})
		})

		// Unsplash image proxy/search
		api.GET("/unsplash", unsplashHandler)
		// IATA code 查詢
		api.POST("/iata", getIATACode)
	}

	// 啟動伺服器
	port := ":8080"
	log.Printf("Server running on http://localhost%s", port)
	log.Printf("Frontend: http://localhost%s/web", port)
	log.Printf("API: http://localhost%s/api", port)
	if err := r.Run(port); err != nil {
		log.Fatal(err)
	}
}
