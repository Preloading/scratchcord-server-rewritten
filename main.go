package main

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
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
	DateCreated uint64
	LastLogin   uint64
	Ranks       string
}

const (
	hash_default_cost int = 14
)

var (
	// Obviously, this is just a test example. Do not do this in production.
	// In production, you would have the private key and public key pair generated
	// in advance. NEVER add a private key to any GitHub repo.
	privateKey                  *rsa.PrivateKey
	motd                        string   = os.Getenv("SCRATCHCORD_MOTD")
	webhook_url                 string   = os.Getenv("SCRATCHCORD_WEBHOOK_URL")
	admin_password              string   = os.Getenv("SCRATCHCORD_ADMIN_PASSWORD")
	server_url                  string   = os.Getenv("SCRATCHCORD_SERVER_URL") // Example: http://127.0.0.1 or https://example.com/scratchcord/api
	permitted_protocol_versions []string = []string{"SCLPV10", "SCPV10"}
	db                          *gorm.DB
	BroadcastPublisher          = NewEventPublisher()
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
	db.AutoMigrate(&Ranks{})

	// Initialize Ranks
	InitializeRanks()

	// Register default admin account (in order to be able to administer without DB edits)
	register_default_admin_account()

	// Create fiber application
	app := fiber.New(fiber.Config{
		BodyLimit: 4 * 1024 * 1024, // 4MB
	})

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
	app.Get("/get_rank_info", GetRankInfo)

	app.Static("/uploads", "./uploads")

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

	app.Post("/reauth", reauth)
	app.Get("/check_auth", check_auth)
	app.Get("/get_offline_messages/:channel", get_offline_messages)

	// User Management
	app.Post("/change_password", change_password)
	app.Post("/upload_profile_picture", UploadProfilePicture)

	// Admin Requests
	app.Post("/admin/api/grant_rank", GrantRanksAPI)
	app.Post("/admin/apupload_profile_picture/i/revoke_rank", RevokeRanksAPI)

	app.Post("/admin/api/delete_rank", DeleteRankAPI)
	app.Post("/admin/api/create_rank", CreateRankAPI)
	app.Post("/admin/api/reset_password", ChangePasswordAdmin)

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
