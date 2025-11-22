package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// ========== 資料模型 ==========
type Trip struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Region      string      `json:"region"`
	StartDate   string      `json:"start_date"`
	Days        int         `json:"days"`
	BudgetTWD   int         `json:"budget_twd"`
	People      int         `json:"people"`
	DailyHours  int         `json:"daily_hours"`
	Preferences Preferences `json:"preferences"`
	Plan        []Day       `json:"plan"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Preferences struct {
	Pace      string   `json:"pace"`
	Types     []string `json:"types"`
	Transport []string `json:"transport"`
	Dining    []string `json:"dining"`
}

type Day struct {
	DayIndex int    `json:"day_index"`
	Date     string `json:"date"`
	Items    []Item `json:"items"`
}

type Item struct {
	ID          string  `json:"id"`
	Time        string  `json:"time"`
	DurationMin int     `json:"duration_min"`
	Title       string  `json:"title"`
	Address     string  `json:"address"`
	Lat         float64 `json:"lat,omitempty"`
	Lng         float64 `json:"lng,omitempty"`
	Link        string  `json:"link"`
	Note        string  `json:"note"`
}

// ========== 資料存儲 ==========
var (
	trips    = make(map[int]*Trip)
	tripsMux sync.RWMutex
	nextID   = 1
	dataFile = "../data/trips_data.json"
)

// ========== 主程式 ==========
func main() {
	// 載入既有資料
	loadTrips()

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

		// 健康檢查
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "ok",
				"time":   time.Now(),
			})
		})
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

// ========== API Handlers ==========

func getTrips(c *gin.Context) {
	tripsMux.RLock()
	defer tripsMux.RUnlock()

	tripList := make([]*Trip, 0, len(trips))
	for _, t := range trips {
		tripList = append(tripList, t)
	}

	c.JSON(200, tripList)
}

func getTrip(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}

	tripsMux.RLock()
	defer tripsMux.RUnlock()

	trip, exists := trips[id]
	if !exists {
		c.JSON(404, gin.H{"error": "Trip not found"})
		return
	}

	c.JSON(200, trip)
}

func createTrip(c *gin.Context) {
	var trip Trip
	if err := c.ShouldBindJSON(&trip); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	tripsMux.Lock()
	defer tripsMux.Unlock()

	trip.ID = nextID
	trip.CreatedAt = time.Now()
	trip.UpdatedAt = time.Now()

	// 自動展開日期
	if trip.Plan == nil || len(trip.Plan) == 0 {
		trip.Plan = expandDays(trip.StartDate, trip.Days)
	}

	trips[trip.ID] = &trip
	nextID++

	// 儲存到檔案
	saveTrips()

	c.JSON(201, trip)
}

func updateTrip(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}

	var updateData Trip
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	tripsMux.Lock()
	defer tripsMux.Unlock()

	trip, exists := trips[id]
	if !exists {
		c.JSON(404, gin.H{"error": "Trip not found"})
		return
	}

	// 更新欄位
	trip.Name = updateData.Name
	trip.Region = updateData.Region
	trip.StartDate = updateData.StartDate
	trip.Days = updateData.Days
	trip.BudgetTWD = updateData.BudgetTWD
	trip.People = updateData.People
	trip.DailyHours = updateData.DailyHours
	trip.Preferences = updateData.Preferences
	trip.Plan = updateData.Plan
	trip.UpdatedAt = time.Now()

	saveTrips()

	c.JSON(200, trip)
}

func deleteTrip(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}

	tripsMux.Lock()
	defer tripsMux.Unlock()

	if _, exists := trips[id]; !exists {
		c.JSON(404, gin.H{"error": "Trip not found"})
		return
	}

	delete(trips, id)
	saveTrips()

	c.JSON(200, gin.H{"message": "Trip deleted"})
}

// ========== 輔助函數 ==========

func expandDays(startDate string, days int) []Day {
	result := make([]Day, days)
	start, _ := time.Parse("2006-01-02", startDate)

	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i)
		result[i] = Day{
			DayIndex: i + 1,
			Date:     date.Format("2006-01-02"),
			Items:    []Item{},
		}
	}

	return result
}

func loadTrips() {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No existing data file, starting fresh")
			return
		}
		log.Printf("Error loading trips: %v", err)
		return
	}

	var loadedTrips map[int]*Trip
	if err := json.Unmarshal(data, &loadedTrips); err != nil {
		log.Printf("Error parsing trips data: %v", err)
		return
	}

	trips = loadedTrips

	// Find max ID
	maxID := 0
	for id := range trips {
		if id > maxID {
			maxID = id
		}
	}
	nextID = maxID + 1

	log.Printf("Loaded %d trips", len(trips))
}

func saveTrips() {
	data, err := json.MarshalIndent(trips, "", "  ")
	if err != nil {
		log.Printf("Error marshaling trips: %v", err)
		return
	}

	if err := os.WriteFile(dataFile, data, 0644); err != nil {
		log.Printf("Error saving trips: %v", err)
		return
	}
}
