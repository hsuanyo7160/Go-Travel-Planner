package main

import (
	//"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"google.golang.org/genai"
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

		api.POST("/gemini", callGemini)
		api.POST("/gemini/save", saveGeminiToFile)
		api.GET("/gemini/response", getGeminiResponse)

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

// ====== Gemini 呼叫 ======
func callGemini(c *gin.Context) {
	var req struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	model := req.Model
	if model == "" {
		model = "gemini-2.5-flash-lite"
	}

	res, err := client.Models.GenerateContent(ctx, model, genai.Text(req.Prompt), nil)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"text": res.Text()})
}

// saveGeminiToFile 將收到的文字儲存為 data 目錄下的檔案
func saveGeminiToFile(c *gin.Context) {
	var req struct {
		Filename string `json:"filename"`
		Name     string `json:"name"`
		Text     string `json:"text"`
		Format   string `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	name := req.Filename
	if name == "" {
		name = fmt.Sprintf("gemini_%d", time.Now().Unix())
	}

	// 只保留安全字元
	re := regexp.MustCompile(`[^A-Za-z0-9._-]`)
	name = re.ReplaceAllString(name, "_")

	ext := ".txt"
	if strings.ToLower(req.Format) == "json" {
		ext = ".json"
	}

	// 若為 JSON 格式，固定檔名為 response.json（寫入 data/response.json）
	if strings.ToLower(req.Format) == "json" {
		name = "response" + ext
	} else {
		if !strings.HasSuffix(name, ext) {
			name = name + ext
		}
	}

	dest := filepath.Join("../data", name)

	// 若請求要求 JSON 格式，將回應切段並 append 到目標檔案的 response 陣列
	if strings.ToLower(req.Format) == "json" {
		// 不切段：將整段回覆視為 single element，trim 後 append（若為空則不 append）
		var parts []string
		if t := strings.TrimSpace(req.Text); t != "" {
			parts = []string{t}
		} else {
			parts = []string{}
		}

		type OutFile struct {
			Name     string   `json:"name"`
			Response []string `json:"response"`
		}

		var out OutFile
		// 若檔案已存在，讀取並合併
		if b, err := os.ReadFile(dest); err == nil {
			if err := json.Unmarshal(b, &out); err != nil {
				// 若既有檔案不是期望格式，覆寫為新格式
				out = OutFile{Name: req.Name, Response: []string{}}
			}
		} else {
			// 檔案不存在，建立新結構
			out = OutFile{Name: req.Name, Response: []string{}}
		}

		// 若 out.Name 為空，填入 req.Name
		if out.Name == "" {
			out.Name = req.Name
		}

		// Append parts
		out.Response = append(out.Response, parts...)

		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// 寫回檔案（覆蓋）
		if err := os.WriteFile(dest, data, 0644); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"path": dest})
		return
	}

	// 否則當作純文字寫入
	if err := os.WriteFile(dest, []byte(req.Text), 0644); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"path": dest})
}

// getGeminiResponse 讀取 data/response.json 並回傳 JSON 結構
func getGeminiResponse(c *gin.Context) {
	path := filepath.Join("../data", "response.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 回傳空的結構
			c.JSON(200, gin.H{"name": "", "response": []string{}})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		// 若檔案不是合法 JSON，回傳原始文字
		c.JSON(200, gin.H{"raw": string(b)})
		return
	}
	c.JSON(200, out)
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
