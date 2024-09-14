package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/monitor"

	// Database
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	// Auth
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/golang-jwt/jwt/v5"
)

type LoginRequest struct {
	Username string
	Password string
}

type GlobalWebsocketCommand struct {
	Cmd string
}
type SendMessageRequest struct {
	Cmd     string
	Message string
}

var (
	// Obviously, this is just a test example. Do not do this in production.
	// In production, you would have the private key and public key pair generated
	// in advance. NEVER add a private key to any GitHub repo.
	privateKey    *rsa.PrivateKey
	motd          string = "Welcome to scratchcord!"
	db            *gorm.DB
	BroadcastChan = make(chan string)
)

func main() {
	// Configure runtime settings
	debug.SetGCPercent(35) // 35% limit for GC

	// Auth setup
	rng := rand.Reader
	var err error
	privateKey, err = rsa.GenerateKey(rng, 2048)
	if err != nil {
		log.Fatalf("rsa.GenerateKey: %v", err)
	}

	// Database
	db, err = gorm.Open(sqlite.Open("sqlite/scratchcord.db"), &gorm.Config{})

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

	app.Get("/check_auth", check_auth)
	app.Get("/get_offline_messages/:channel", get_offline_messages)

	log.Fatal(app.Listen(":3000"))
	// Access the websocket server: ws://0.0.0.0:3000/

	//log.Fatal(app.ListenTLS("0.0.0.0:3000", "./cert.pem", "./key.pem"))
	// Access the websocket server: wss://0.0.0.0:3000/

}

func (m *Messages) AfterCreate(tx *gorm.DB) (err error) {
	// Serialize the message to JSON
	msgJSON, _ := json.Marshal(map[string]interface{}{
		"event": "new_message",
		"data":  m, // Send the newly created message
	})
	BroadcastChan <- string(msgJSON)
	return
}

func (m *Messages) AfterUpdate(tx *gorm.DB) (err error) {
	// Similar to AfterCreate, serialize and broadcast the updated message
	msgJSON, _ := json.Marshal(map[string]interface{}{
		"event": "message_updated",
		"data":  m,
	})
	BroadcastChan <- string(msgJSON)
	return
}

func hello(c *fiber.Ctx) error {
	// Variable is only valid within this handler
	return c.SendString("Hello, World!")
}

func login(c *fiber.Ctx) error {
	r := new(LoginRequest)

	if err := json.Unmarshal(c.BodyRaw(), &r); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	// Create the Claims
	claims := jwt.MapClaims{
		"username":  r.Username,
		"supporter": true, // This is temporary for before we get a DB
		"exp":       time.Now().Add(time.Hour * 72).Unix(),
	}
	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Generate encoded token and send it as response.
	t, err := token.SignedString(privateKey)
	if err != nil {
		log.Printf("token.SignedString: %v", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.JSON(fiber.Map{"token": t, "motd": motd})
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
	username := claims["username"].(string)

	// websocket.Conn bindings https://pkg.go.dev/github.com/fasthttp/websocket?tab=doc#pkg-index
	var (
		mt  int
		msg []byte
		err error
	)
	// Start a goroutine to listen for new messages
	go func() {
		for {
			msg := <-BroadcastChan
			if err := c.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
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
		log.Printf("recv: %s", msg)

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
			db_msg := Messages{Message: string(msg), Username: username, Channel: channel, Timestamp: uint64(time.Now().Unix())}
			db.Create(&db_msg)
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
	db.Order("timestamp DESC").Limit(30).Find(&offline_messages, "channel = ?", channel)
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
