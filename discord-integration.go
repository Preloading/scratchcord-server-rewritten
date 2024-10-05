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
			// 1 - Normal Message
			// 2 - Nudge
			// 3 - Typing, does not store in DB
			// 4 - Game Start Request
			// 100 - Global Message (admin only)
			// 101 - Global TTS Message (admin only)
			// 102 - Channel Special Message (admin only)
			// 103 - Channel Special TTS Message (admin only)
			// 104 - Kick User (admin only)
			// 105 - Kick All Users (admin only)
			var webhookUsername string
			var webhookContents string
			switch msg.data.Type {
			case 1:
				webhookUsername = user.Username
				webhookContents = msg.data.Message
			case 2:
				webhookUsername = user.Username
				webhookContents = user.Username + " has sent a nudge!"
			case 100:
			case 101:
			case 102:
			case 103:
				webhookUsername = "System Message"
				webhookContents = user.Username + ": " + msg.data.Message
			case 3:
			case 4:
			case 104:
			case 105:
				continue
			}
			message := discordwebhook.Message{
				Username: &webhookUsername,
				//AvatarUrl: &user.Avatar,
				Content: &webhookContents,
			}
			if err := discordwebhook.SendMessage(webhook_url, message); err != nil {
				log.Println(err)
			}
		}
	}()
}
