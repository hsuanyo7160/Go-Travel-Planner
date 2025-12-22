package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ========== MongoDB ==========
var mongoClient *mongo.Client
var tripsCollection *mongo.Collection

func initMongo() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("MongoDB connect error: %v", err)
	}

	// 可以決定 db / collection 名稱
	mongoClient = client
	tripsCollection = client.Database("go_travel").Collection("trips")

	log.Println("MongoDB connected")
}
