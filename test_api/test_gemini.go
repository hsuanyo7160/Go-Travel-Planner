package main

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/genai"
)

func test_gemini() {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash-lite",
		genai.Text("Explain how AI works in a few words"),
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Text())
}

//
