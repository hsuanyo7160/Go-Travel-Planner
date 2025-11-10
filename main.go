package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

/* ==================== 資料模型 ==================== */

type Item struct {
	ID          string  `json:"id"`
	Time        string  `json:"time"`         // "HH:MM"
	DurationMin int     `json:"duration_min"` // 分鐘
	Title       string  `json:"title"`
	Address     string  `json:"address"`
	Lat         float64 `json:"lat,omitempty"`
	Lng         float64 `json:"lng,omitempty"`
	Link        string  `json:"link"`
	Note        string  `json:"note"`
}

type Day struct {
	DayIndex int    `json:"day_index"`
	Date     string `json:"date"` // "YYYY-MM-DD"
	Items    []Item `json:"items"`
}

type Trip struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Region     string    `json:"region"`
	StartDate  string    `json:"start_date"` // "YYYY-MM-DD"
	Days       int       `json:"days"`
	BudgetTWD  int       `json:"budget_twd"`
	People     int       `json:"people"`
	DailyHours int       `json:"daily_hours"`
	Plan       []Day     `json:"plan"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

/* ==================== 全域狀態 ==================== */

var (
	dataFile = "./trips.json"
	trips    = make(map[int]*Trip)
	mtx      sync.RWMutex
	nextID   = 1
	locTZ, _ = time.LoadLocation("Asia/Taipei")
)

/* ==================== 主程式 ==================== */

func main() {
	// 讀檔（若沒有就建一個空檔）
	if err := load(); err != nil {
		fmt.Println("load error:", err)
	}

	r := gin.Default()

	// CORS：允許從 5500（前端本機開伺服器）與 8080 同源請求
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5500", "http://127.0.0.1:5500", "http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// 健康檢查
	r.GET("/check", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"time": time.Now().In(locTZ).Format(time.RFC3339),
		})
	})

	// 前端靜態檔（把你的 index.html 放 ./static/）
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/web/") })
	r.Static("/web", "./static")

	// ====== 旅遊助手 API ======
	api := r.Group("/api")
	{
		// ★新增：列出全部行程（給前端下拉選單用）
		api.GET("/trips", func(c *gin.Context) {
			mtx.RLock()
			defer mtx.RUnlock()
			list := make([]*Trip, 0, len(trips))
			for _, t := range trips {
				list = append(list, t)
			}
			sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
			c.JSON(http.StatusOK, list)
		})

		// 建立行程（依 start_date + days 自動展開 Day1..N）
		api.POST("/trips", func(c *gin.Context) {
			var req struct {
				Name       string `json:"name" binding:"required"`
				Region     string `json:"region" binding:"required"`
				StartDate  string `json:"start_date" binding:"required"`
				Days       int    `json:"days" binding:"required"`
				BudgetTWD  int    `json:"budget_twd"`
				People     int    `json:"people"`
				DailyHours int    `json:"daily_hours"`
			}
			if err := c.ShouldBindJSON(&req); err != nil || req.Days <= 0 || !isISODate(req.StartDate) {
				c.String(http.StatusBadRequest, "bad payload")
				return
			}
			if req.People <= 0 {
				req.People = 1
			}
			if req.DailyHours <= 0 {
				req.DailyHours = 8
			}

			mtx.Lock()
			defer mtx.Unlock()

			t := &Trip{
				ID:         nextID,
				Name:       strings.TrimSpace(req.Name),
				Region:     strings.TrimSpace(req.Region),
				StartDate:  req.StartDate,
				Days:       req.Days,
				BudgetTWD:  req.BudgetTWD,
				People:     req.People,
				DailyHours: req.DailyHours,
				Plan:       expandDays(req.StartDate, req.Days),
				CreatedAt:  time.Now().In(locTZ),
				UpdatedAt:  time.Now().In(locTZ),
			}
			trips[t.ID] = t
			nextID++
			if err := save(); err != nil {
				c.String(http.StatusInternalServerError, "persist failed: %v", err)
				return
			}
			c.JSON(http.StatusCreated, t)
		})

		// 讀單一行程
		api.GET("/trips/:id", func(c *gin.Context) {
			id, _ := strconv.Atoi(c.Param("id"))
			mtx.RLock()
			defer mtx.RUnlock()
			if t, ok := trips[id]; ok {
				c.JSON(http.StatusOK, t)
				return
			}
			c.Status(http.StatusNotFound)
		})

		// 更新整個行程（基本資料或整包 plan）
		api.PUT("/trips/:id", func(c *gin.Context) {
			id, _ := strconv.Atoi(c.Param("id"))
			var body Trip
			if err := c.ShouldBindJSON(&body); err != nil || body.Days <= 0 || !isISODate(body.StartDate) {
				c.String(http.StatusBadRequest, "bad payload")
				return
			}

			mtx.Lock()
			defer mtx.Unlock()

			t, ok := trips[id]
			if !ok {
				c.Status(http.StatusNotFound)
				return
			}

			t.Name = strings.TrimSpace(body.Name)
			t.Region = strings.TrimSpace(body.Region)
			t.StartDate = body.StartDate
			t.Days = body.Days
			t.BudgetTWD = body.BudgetTWD
			t.People = body.People
			t.DailyHours = body.DailyHours

			// 若傳入 plan 就依 day_index 對齊，否則重展開
			if len(body.Plan) > 0 {
				out := expandDays(t.StartDate, t.Days)
				for i := range out {
					for _, d := range body.Plan {
						if d.DayIndex == out[i].DayIndex {
							out[i].Items = d.Items
							break
						}
					}
				}
				t.Plan = out
			} else {
				t.Plan = expandDays(t.StartDate, t.Days)
			}

			t.UpdatedAt = time.Now().In(locTZ)
			if err := save(); err != nil {
				c.String(http.StatusInternalServerError, "persist failed: %v", err)
				return
			}
			c.JSON(http.StatusOK, t)
		})

		// 刪除行程
		api.DELETE("/trips/:id", func(c *gin.Context) {
			id, _ := strconv.Atoi(c.Param("id"))
			mtx.Lock()
			defer mtx.Unlock()
			if _, ok := trips[id]; !ok {
				c.Status(http.StatusNotFound)
				return
			}
			delete(trips, id)
			if err := save(); err != nil {
				c.String(http.StatusInternalServerError, "persist failed: %v", err)
				return
			}
			c.Status(http.StatusNoContent)
		})
	}

	const addr = ":8080"
	fmt.Println("Server on", addr)
	if err := r.Run(addr); err != nil {
		panic(err)
	}
}

/* ==================== 工具/邏輯 ==================== */

// 展開 Day1...DayN 的日期
func expandDays(start string, n int) []Day {
	res := make([]Day, 0, n)
	base, _ := time.ParseInLocation("2006-01-02", start, locTZ)
	for i := 0; i < n; i++ {
		d := base.AddDate(0, 0, i).Format("2006-01-02")
		res = append(res, Day{DayIndex: i + 1, Date: d, Items: []Item{}})
	}
	return res
}

func isISODate(s string) bool {
	_, err := time.ParseInLocation("2006-01-02", s, locTZ)
	return err == nil
}

/* ==================== 檔案存取 ==================== */

func load() error {
	f, err := os.Open(dataFile)
	if errors.Is(err, os.ErrNotExist) {
		if err := persist(map[int]*Trip{}); err != nil {
			return err
		}
		trips = map[int]*Trip{}
		nextID = 1
		return nil
	} else if err != nil {
		return err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var m map[int]*Trip
	if err := dec.Decode(&m); err != nil {
		return err
	}
	trips = m

	// 讓 nextID 延續
	maxID := 0
	for id := range trips {
		if id > maxID {
			maxID = id
		}
	}
	nextID = maxID + 1
	return nil
}

func save() error {
	return persist(trips)
}

func persist(data map[int]*Trip) error {
	tmp := dataFile + ".tmp"
	if err := os.MkdirAll(filepath.Dir(dataFile), 0o755); err != nil && !os.IsExist(err) {
		return err
	}
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmp, dataFile)
}
