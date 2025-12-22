package main

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

func getTrips(c *gin.Context) {
	cursor, err := tripsCollection.Find(context.Background(), bson.M{})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(context.Background())

	var tripList []Trip
	for cursor.Next(context.Background()) {
		var t Trip
		if err := cursor.Decode(&t); err == nil {
			tripList = append(tripList, t)
		}
	}

	c.JSON(200, tripList)
}

func getTrip(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}

	var trip Trip
	err = tripsCollection.FindOne(context.Background(), bson.M{"id": id}).Decode(&trip)
	if err != nil {
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

	trip.ID = int(time.Now().Unix())
	trip.CreatedAt = time.Now()
	trip.UpdatedAt = time.Now()

	if trip.Plan == nil || len(trip.Plan) == 0 {
		trip.Plan = expandDays(trip.StartDate, trip.Days)
	}

	_, err := tripsCollection.InsertOne(context.Background(), trip)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, trip)
}

func updateTrip(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}

	// 1. 先用 map 接收前端傳來的資料，這樣才能知道前端「到底傳了哪些欄位」
	var rawMap map[string]interface{}
	if err := c.ShouldBindJSON(&rawMap); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 2. 準備要更新的 bson map
	update := bson.M{
		"updated_at": time.Now(),
	}

	// 3. 逐一檢查欄位，有傳才更新
	if v, ok := rawMap["name"]; ok {
		update["name"] = v
	}
	if v, ok := rawMap["region"]; ok {
		update["region"] = v
	}
	if v, ok := rawMap["start_date"]; ok {
		update["start_date"] = v
	}
	if v, ok := rawMap["days"]; ok {
		update["days"] = v
	}
	if v, ok := rawMap["budget_twd"]; ok {
		update["budget_twd"] = v
	}
	if v, ok := rawMap["people"]; ok {
		update["people"] = v
	}
	if v, ok := rawMap["daily_hours"]; ok {
		update["daily_hours"] = v
	}
	if v, ok := rawMap["preferences"]; ok {
		update["preferences"] = v
	}

	// ⚠️ 關鍵：只有當前端真的傳了 "plan" 欄位時，才去更新它
	// 如果前端沒傳 (因為是微調模式)，這裡就不會把 plan 覆蓋掉
	if v, ok := rawMap["plan"]; ok {
		update["plan"] = v
	}

	// 4. 執行更新
	result, err := tripsCollection.UpdateOne(
		context.Background(),
		bson.M{"id": id},
		bson.M{"$set": update},
	)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(404, gin.H{"error": "Trip not found"})
		return
	}

	c.JSON(200, gin.H{"message": "Trip updated"})
}

func deleteTrip(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}

	_, err = tripsCollection.DeleteOne(context.Background(), bson.M{"id": id})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Trip deleted"})
}
