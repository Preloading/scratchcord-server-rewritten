package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	// Crypto Stuff

	// Database

	// Auth
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/golang-jwt/jwt/v5"
)

type GlobalWebsocketCommand struct {
	Cmd string
}
type SendMessageRequest struct {
	Cmd     string
	Message string
}
type RecievedMessageResponse struct {
	Cmd       string
	UserId    uint
	MessageId uint
	Message   string
}
type RecievedMessageResponseNoBody struct {
	Cmd       string
	UserId    uint
	MessageId uint
}
type RecievedTypingResponse struct {
	Cmd    string
	UserId uint
}
type KickedResponse struct {
	Cmd     string
	Message string
}
type BroadcastDBMessage struct {
	event string
	data  Messages
}
type UserInfoResponse struct {
	ID          uint
	Username    string
	Avatar      string
	Supporter   bool
	DateCreated uint64
	LastLogin   uint64
}

const (
	hash_default_cost int = 14
)

var (
	// Obviously, this is just a test example. Do not do this in production.
	// In production, you would have the private key and public key pair generated
	// in advance. NEVER add a private key to any GitHub repo.
	privateKey         *rsa.PrivateKey
	motd               string = os.Getenv("SCRATCHCORD_MOTD")
	webhook_url        string = os.Getenv("SCRATCHCORD_WEBHOOK_URL")
	db                 *gorm.DB
	BroadcastPublisher = NewEventPublisher()
)

func main() {
	// Configure runtime settings
	debug.SetGCPercent(35) // 35% limit for GC

	godotenv.Load()

	// Auth setup
	rng := rand.Reader
	var err error
	privateKey, err = rsa.GenerateKey(rng, 2048)
	if err != nil {
		log.Fatalf("rsa.GenerateKey: %v", err)
	}

	// Database
	os.Getenv("SCRATCHCORD_DB_PATH")
	if _, err := os.Stat(os.Getenv("SCRATCHCORD_DB_PATH")); errors.Is(err, os.ErrNotExist) {
		os.Create(os.Getenv("SCRATCHCORD_DB_PATH"))
	}

	db, err = gorm.Open(sqlite.Open(os.Getenv("SCRATCHCORD_DB_PATH")), &gorm.Config{})

	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&Messages{})
	db.AutoMigrate(&Accounts{})

	// Create fiber application
	app := fiber.New()

	// app.Use(cors.New())
	app.Use(cors.New(cors.Config{
		AllowCredentials: true,
		AllowOrigins:     "https://studio.penguinmod.com",
		AllowHeaders:     "Origin, Content-Type, Accept, Accept-Language, Content-Length, Authorization",
	}))

	// Paths
	app.Get("/hello", hello)
	app.Post("/login", login)
	app.Post("/register", register)

	app.Get("/get_user_info", get_user_info)

	// Add a websocket path
	app.Use("/ws", websockek_path)
	app.Get("/ws/:channel", websocket.New(global_channel_websocket_handler))

	app.Get("/monitor", monitor.New(
		monitor.Config{
			Title:   "Metrics",
			Refresh: (50 * time.Millisecond),
		},
	))
	// Handling Authenticated Points

	// JWT Middleware
	app.Use(jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{
			JWTAlg: jwtware.RS256,
			Key:    privateKey.Public(),
		},
	}))

	start_discord_webhook() // Start the discord webhook

	app.Get("/check_auth", check_auth)
	app.Get("/get_offline_messages/:channel", get_offline_messages)

	log.Fatal(app.Listen(":3000"))
	// Access the websocket server: ws://0.0.0.0:3000/

	//log.Fatal(app.ListenTLS("0.0.0.0:3000", "./cert.pem", "./key.pem"))
	// Access the websocket server: wss://0.0.0.0:3000/
}

func (m *Messages) AfterCreate(tx *gorm.DB) (err error) {
	// Serialize the message to JSON
	msg := BroadcastDBMessage{
		event: "new_message",
		data:  *m,
	}
	BroadcastPublisher.Publish(msg)
	return
}

func (m *Messages) AfterUpdate(tx *gorm.DB) (err error) {
	// Similar to AfterCreate, broadcast the updated message
	msg := BroadcastDBMessage{
		event: "message_updated",
		data:  *m,
	}
	BroadcastPublisher.Publish(msg)
	return
}

func hello(c *fiber.Ctx) error {
	// Variable is only valid within this handler
	return c.SendString("Hello, World!")
}
func get_user_info(c *fiber.Ctx) error {
	// user := c.Locals("user").(*jwt.Token)
	// if !user.Valid {
	// 	return c.SendString("Invalid Token!")
	// }
	user := Accounts{}
	userid := c.Query("userid")
	username := c.Query("username")
	if userid != "" {
		db.First(&user, "id = ?", userid)
	} else if username != "" {
		db.First(&user, "username = ?", username)
	} else {
		return c.SendString("malformed input!")
	}
	if user.ID == 0 {
		return c.SendString("user not found!")
	}
	userResponse := UserInfoResponse{
		ID:          user.ID,
		Username:    user.Username,
		Avatar:      user.Avatar,
		Supporter:   user.Supporter,
		DateCreated: user.DateCreated,
		LastLogin:   user.LastLogin,
	}
	return c.JSON(userResponse)
}

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

	//username := account.Username

	// websocket.Conn bindings https://pkg.go.dev/github.com/fasthttp/websocket?tab=doc#pkg-index
	var (
		mt  int
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
				responce := RecievedMessageResponse{
					Cmd:       "recv_msg",
					UserId:    recv_msg.data.UserId,
					MessageId: recv_msg.data.ID,
					Message:   recv_msg.data.Message,
				}
				responce_json, err = json.Marshal(responce)
			case 2: // Nudge
				responce := RecievedMessageResponseNoBody{
					Cmd:       "recv_nudge",
					UserId:    recv_msg.data.UserId,
					MessageId: recv_msg.data.ID,
				}
				responce_json, err = json.Marshal(responce)
			case 3:
				responce := RecievedMessageResponseNoBody{
					Cmd:    "recv_typing",
					UserId: recv_msg.data.UserId,
				}
				responce_json, err = json.Marshal(responce)
			case 100:
			case 102:
				responce := RecievedMessageResponse{
					Cmd:       "recv_special_msg",
					UserId:    recv_msg.data.UserId,
					MessageId: recv_msg.data.ID,
					Message:   recv_msg.data.Message,
				}
				responce_json, err = json.Marshal(responce)
			case 101:
			case 103:
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
		if mt, msg, err = c.ReadMessage(); err != nil {
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
			r := SendMessageRequest{}
			if err := json.Unmarshal(msg, &r); err != nil {
				c.Close()
				return
			}
			log.Println(r.Message)
			db_msg := Messages{
				Message:   string(r.Message),
				UserId:    uint(user_id),
				Type:      1,
				Channel:   channel,
				Timestamp: uint64(time.Now().Unix()),
			}
			db.Create(&db_msg)
		case "nudge":
			db_msg := Messages{
				Message:   "",
				UserId:    uint(user_id),
				Type:      2,
				Channel:   channel,
				Timestamp: uint64(time.Now().Unix()),
			}
			db.Create(&db_msg)
		case "typing":
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

		if err = c.WriteMessage(mt, msg); err != nil {
			log.Println("write:", err)
			break
		}
		// if err = c.WriteMessage(mt, []byte("Hello "+username+" welcome to #"+channel)); err != nil {
		// 	log.Println("write:", err)
		// 	break
		// }
	}
}
func check_auth(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	name := claims["username"].(string)
	return c.SendString("Welcome " + name)
}

func get_offline_messages(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	if !user.Valid {
		return c.SendString("Invalid Token!")
	}
	channel := c.Params("channel")
	if channel == "" {
		return c.SendString("Invalid Channel!")
	}

	offline_messages := []Messages{}
	db.Order("timestamp ASC").Limit(30).Find(&offline_messages, "channel = ?", channel)
	return c.JSON(offline_messages)
}

func websockek_path(c *fiber.Ctx) error {
	// IsWebSocketUpgrade returns true if the client
	// requested upgrade to the WebSocket protocol.
	if websocket.IsWebSocketUpgrade(c) {
		// c.Locals("allowed", true)
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}
