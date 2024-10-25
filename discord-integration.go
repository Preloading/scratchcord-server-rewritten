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
			// 5 - Message with TTS
			// 6 - Game Join

			// 100 - Global Message (admin only)
			// 101 - Global TTS Message (admin only)
			// 102 - Channel Special Message (admin only)
			// 103 - Channel Special TTS Message (admin only)
			// 104 - Kick User (admin only)
			// 105 - Kick All Users (admin only)
			var webhookUsername string
			// Avatar might not work on localhost
			var webhookAvatar string
			var webhookContents string
			switch msg.data.Type {
			case 1, 5:
				webhookUsername = user.Username
				webhookAvatar = user.Avatar
				webhookContents = msg.data.Message
			case 2:
				webhookUsername = user.Username
				webhookAvatar = user.Avatar
				webhookContents = user.Username + " has sent a nudge!"
			case 100, 101, 102, 103:
				webhookUsername = "System Message"
				webhookAvatar = user.Avatar
				webhookContents = user.Username + ": " + msg.data.Message
			case 3, 4, 6, 104, 105:
				continue
			}
			message := discordwebhook.Message{
				Username:  &webhookUsername,
				AvatarUrl: &webhookAvatar,
				Content:   &webhookContents,
			}
			if err := discordwebhook.SendMessage(webhook_url, message); err != nil {
				log.Println(err)
			}
		}
	}()
}
