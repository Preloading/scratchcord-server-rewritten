package main

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/golang-jwt/jwt/v5"
)

func global_channel_websocket_handler(c *websocket.Conn) {
	// c.Locals is added to the *websocket.Conn

	// Handle authentication
	channel := c.Params("channel")
	if c.Query("token") == "" {
		fmt.Println("no token provided!")
		c.Close()
		return
	}
	fmt.Println(c.Query("token"))
	token, tokenerr := jwt.Parse(c.Query("token"), func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", c.Query("token"))
		}
		return privateKey.Public(), nil
	})

	if tokenerr != nil {
		fmt.Println("error on decoding token: " + tokenerr.Error())
		c.Close()
		return
	}

	if !token.Valid {
		fmt.Println("token invalid!")
		c.Close()
		return
	}

	claims := token.Claims.(jwt.MapClaims)
	user_id := claims["id"].(float64)

	account := Accounts{}
	if err := db.First(&account, "id = ?", user_id); err.Error != nil {
		c.Close()
		return
	}
	ranks, rankerr := GetEffectivePermissions(account.Ranks)
	if rankerr != nil {
		c.Close()
		return
	}
	//username := account.Username

	// websocket.Conn bindings https://pkg.go.dev/github.com/fasthttp/websocket?tab=doc#pkg-index
	var (
		// mt  int
		msg []byte
		err error
	)
	// Start a goroutine to listen for new messages
	go func() {
		eventChannel := BroadcastPublisher.Subscribe()
		defer BroadcastPublisher.Unsubscribe(eventChannel)
		for recv_msg := range eventChannel {
			// If the message is of a type that is global, we ignore the channel check
			if recv_msg.data.Type == 100 ||
				recv_msg.data.Type == 101 ||
				recv_msg.data.Type == 105 {

			} else if recv_msg.data.Channel != channel {
				continue
			}

			// Handle kicking a specific user
			if recv_msg.data.Type == 104 && recv_msg.data.UserId != uint(user_id) {
				continue
			}
			responce_json := []byte{}
			switch recv_msg.data.Type {
			case 1: // Normal Message
				if !slices.Contains(ranks, "CanReadMessages") {
					break
				}
				responce := RecievedMessageResponse{
					Cmd:       "recv_msg",
					UserId:    recv_msg.data.UserId,
					MessageId: recv_msg.data.ID,
					Message:   recv_msg.data.Message,
				}
				responce_json, err = json.Marshal(responce)
			case 2: // Nudge
				if !slices.Contains(ranks, "CanReadNudges") {
					break
				}
				responce := RecievedMessageResponseNoBody{
					Cmd:       "recv_nudge",
					UserId:    recv_msg.data.UserId,
					MessageId: recv_msg.data.ID,
				}
				responce_json, err = json.Marshal(responce)
			case 3:
				if !slices.Contains(ranks, "CanRecieveTyping") {
					break
				}
				responce := RecievedMessageResponseNoBody{
					Cmd:    "recv_typing",
					UserId: recv_msg.data.UserId,
				}
				responce_json, err = json.Marshal(responce)
			case 100:
			case 102:
				if !slices.Contains(ranks, "CanReadSpecialMessages") {
					break
				}
				responce := RecievedMessageResponse{
					Cmd:       "recv_special_msg",
					UserId:    recv_msg.data.UserId,
					MessageId: recv_msg.data.ID,
					Message:   recv_msg.data.Message,
				}
				responce_json, err = json.Marshal(responce)
			case 101:
			case 103:
				if !slices.Contains(ranks, "CanReadSpecialMessages") {
					break
				}
				responce := RecievedMessageResponse{
					Cmd:       "recv_special_tts_msg",
					UserId:    recv_msg.data.UserId,
					MessageId: recv_msg.data.ID,
					Message:   recv_msg.data.Message,
				}
				responce_json, err = json.Marshal(responce)
			case 104:
			case 105:
				responce := KickedResponse{
					Cmd:     "kicked",
					Message: recv_msg.data.Message,
				}
				responce_json, err = json.Marshal(responce)
				c.WriteMessage(websocket.CloseAbnormalClosure, responce_json)
			default:
				c.Close()
				return
			}

			if err != nil {
				c.Close()
				return
			}
			if err := c.WriteMessage(websocket.TextMessage, responce_json); err != nil {
				log.Println("write error:", err)
				return // Exit the goroutine if there's a write error
			}
		}
	}()

	for {
		if _, msg, err = c.ReadMessage(); err != nil {
			log.Println("read:", err)
			break
		}
		// log.Printf("recv: %s", msg)

		r := GlobalWebsocketCommand{}
		if err := json.Unmarshal(msg, &r); err != nil {
			c.Close()
			return
		}

		switch r.Cmd {
		case "msg":
			if !slices.Contains(ranks, "CanSendMessage") {
				break
			}
			r := SendMessageRequest{}
			if err := json.Unmarshal(msg, &r); err != nil {
				c.Close()
				return
			}
			db_msg := Messages{
				Message:   string(r.Message),
				UserId:    uint(user_id),
				Type:      1,
				Channel:   channel,
				Timestamp: uint64(time.Now().Unix()),
			}
			db.Create(&db_msg)
		case "nudge":
			if !slices.Contains(ranks, "CanSendNudge") {
				break
			}
			db_msg := Messages{
				Message:   "",
				UserId:    uint(user_id),
				Type:      2,
				Channel:   channel,
				Timestamp: uint64(time.Now().Unix()),
			}
			db.Create(&db_msg)
		case "typing":
			if !slices.Contains(ranks, "CanSendTyping") {
				break
			}
			msg := BroadcastDBMessage{
				event: "new_message",
				data: Messages{
					Message:   "",
					UserId:    uint(user_id),
					Type:      3,
					Channel:   channel,
					Timestamp: uint64(time.Now().Unix()),
				},
			}
			BroadcastPublisher.Publish(msg)
		default:
			c.Close()
		}
		// if err = c.WriteMessage(mt, msg); err != nil {
		// 	log.Println("write:", err)
		// 	break
		// }
		// if err = c.WriteMessage(mt, []byte("Hello "+username+" welcome to #"+channel)); err != nil {
		// 	log.Println("write:", err)
		// 	break
		// }
	}
}
