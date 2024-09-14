package main

import (
	"log"

	"github.com/gtuk/discordwebhook"
)

func start_discord_webhook() {
	// Start a goroutine to listen for new messages
	go func() {
		eventChannel := BroadcastPublisher.Subscribe()
		for msg := range eventChannel {
			if msg.data.Channel != "general" {
				continue
			}

			user := Accounts{}
			db.First(&user, "id = ?", msg.data.UserId)

			message := discordwebhook.Message{
				Username: &user.Username,
				//AvatarUrl: &user.Avatar,
				Content: &msg.data.Message,
			}
			if err := discordwebhook.SendMessage(webhook_url, message); err != nil {
				log.Fatal(err)
			}
		}
	}()
}
