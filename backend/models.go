package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ========== 資料模型 ==========
type Trip struct {
	MongoID primitive.ObjectID `bson:"_id,omitempty" json:"-"`

	ID          int         `json:"id" bson:"id"`
	Name        string      `json:"name" bson:"name"`
	Region      string      `json:"region" bson:"region"`
	StartDate   string      `json:"start_date" bson:"start_date"`
	Days        int         `json:"days" bson:"days"`
	BudgetTWD   int         `json:"budget_twd" bson:"budget_twd"`
	People      int         `json:"people" bson:"people"`
	DailyHours  int         `json:"daily_hours" bson:"daily_hours"`
	Preferences Preferences `json:"preferences" bson:"preferences"`
	Plan        []Day       `json:"plan" bson:"plan"`
	CreatedAt   time.Time   `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" bson:"updated_at"`
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

// ChatRequest 前端傳來的請求格式
type ChatRequest struct {
	Message string     `json:"message"` // 使用者這次說的話
	History []ChatPart `json:"history"` // 過去的對話歷史 (可選)
}

// ChatPart 對話歷史的單一則訊息
type ChatPart struct {
	Role string `json:"role"` // "user" (使用者) 或 "model" (AI)
	Text string `json:"text"` // 訊息內容
}
