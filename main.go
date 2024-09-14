package main

import (
	// Crypto Stuff
	"crypto/rand"
	"crypto/rsa"

	"golang.org/x/crypto/bcrypt"

	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gtuk/discordwebhook"

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

type RegisterRequest struct { // This is the same for now, but maybe later more info will be added, like email? idk. Least it'll have the option
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
type RecievedMessageResponse struct {
	Cmd     string
	UserId  string
	Message string
}
type BroadcastDBMessage struct {
	event string
	data  Messages
}

const (
	hash_default_cost int = 14
)

var (
	// Obviously, this is just a test example. Do not do this in production.
	// In production, you would have the private key and public key pair generated
	// in advance. NEVER add a private key to any GitHub repo.
	privateKey    *rsa.PrivateKey
	motd          string = "Welcome to scratchcord!"
	webhook_url   string = "https://discord.com/api/webhooks/1284345712632664160/WAAxnW3-7hoVfslK4SHSv7YnvXaHRCBKiWqXdc_5drJkobzFoLCPQM_GIWh85JRT_U3l"
	db            *gorm.DB
	BroadcastChan = make(chan BroadcastDBMessage)
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
	app.Post("/register", register)

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

	start_discord_webhook()

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
	BroadcastChan <- msg
	BroadcastChan <- msg
	log.Println("Msg Recv")
	return
}

func (m *Messages) AfterUpdate(tx *gorm.DB) (err error) {
	// Similar to AfterCreate, broadcast the updated message
	msg := BroadcastDBMessage{
		event: "message_updated",
		data:  *m,
	}
	BroadcastChan <- msg
	BroadcastChan <- msg
	return
}

func start_discord_webhook() {
	// Start a goroutine to listen for new messages
	go func() {
		for {
			msg := <-BroadcastChan
			// var content = "This is a test message"
			message := discordwebhook.Message{
				Username: &msg.data.Username,
				Content:  &msg.data.Message,
			}
			log.Println("Discord Msg")
			if err := discordwebhook.SendMessage(webhook_url, message); err != nil {
				log.Fatal(err)
			}
		}
	}()
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

	account := Accounts{}
	if err := db.First(&account, "username = ?", r.Username); err.Error != nil {
		return c.SendString("account does not exist!")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(r.Password)); err != nil {
		return c.SendString("wrong password!")
	}

	// Create the Claims (info encoded inside the token)
	claims := jwt.MapClaims{
		"id":  account.ID,
		"exp": time.Now().Add(time.Hour * 72).Unix(),
	}
	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Generate encoded token and send it as response.
	t, err := token.SignedString(privateKey)
	if err != nil {
		log.Printf("token.SignedString: %v", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	// update last login
	account.LastLogin = uint64(time.Now().Unix())
	db.Save(&account)

	return c.JSON(fiber.Map{"token": t, "avatar": account.Avatar, "supporter": account.Supporter, "motd": motd})
}

func register(c *fiber.Ctx) error {
	r := new(RegisterRequest)

	if err := json.Unmarshal(c.BodyRaw(), &r); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	// Check if username is already claimed.
	var count int64 = 0
	db.Model(&Accounts{}).Where("Username = ?", r.Username).Count(&count)
	if count > 0 {
		return c.SendString("username taken!")
	}

	// Generate Password
	hash, err := bcrypt.GenerateFromPassword([]byte(r.Password), hash_default_cost)
	if err != nil {
		return c.SendString("password invalid!")
	}

	account := Accounts{
		Username:     r.Username,
		PasswordHash: string(hash),
		Supporter:    false,
		// Temp avatar till i get proper storage running
		Avatar:      `data:image/svg+xml;utf8,%3Csvg xmlns%3D"http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg" viewBox%3D"0 0 100 100"%3E%3Cmetadata xmlns%3Ardf%3D"http%3A%2F%2Fwww.w3.org%2F1999%2F02%2F22-rdf-syntax-ns%23" xmlns%3Axsi%3D"http%3A%2F%2Fwww.w3.org%2F2001%2FXMLSchema-instance" xmlns%3Adc%3D"http%3A%2F%2Fpurl.org%2Fdc%2Felements%2F1.1%2F" xmlns%3Adcterms%3D"http%3A%2F%2Fpurl.org%2Fdc%2Fterms%2F"%3E%3Crdf%3ARDF%3E%3Crdf%3ADescription%3E%3Cdc%3Atitle%3EInitials%3C%2Fdc%3Atitle%3E%3Cdc%3Acreator%3EDiceBear%3C%2Fdc%3Acreator%3E%3Cdc%3Asource xsi%3Atype%3D"dcterms%3AURI"%3Ehttps%3A%2F%2Fgithub.com%2Fdicebear%2Fdicebear%3C%2Fdc%3Asource%3E%3Cdcterms%3Alicense xsi%3Atype%3D"dcterms%3AURI"%3Ehttps%3A%2F%2Fcreativecommons.org%2Fpublicdomain%2Fzero%2F1.0%2F%3C%2Fdcterms%3Alicense%3E%3Cdc%3Arights%3E%E2%80%9EInitials%E2%80%9D (https%3A%2F%2Fgithub.com%2Fdicebear%2Fdicebear) by %E2%80%9EDiceBear%E2%80%9D%2C licensed under %E2%80%9ECC0 1.0%E2%80%9D (https%3A%2F%2Fcreativecommons.org%2Fpublicdomain%2Fzero%2F1.0%2F)%3C%2Fdc%3Arights%3E%3C%2Frdf%3ADescription%3E%3C%2Frdf%3ARDF%3E%3C%2Fmetadata%3E%3Cmask id%3D"b0esx9i5"%3E%3Crect width%3D"100" height%3D"100" rx%3D"0" ry%3D"0" x%3D"0" y%3D"0" fill%3D"%23fff" %2F%3E%3C%2Fmask%3E%3Cg mask%3D"url(%23b0esx9i5)"%3E%3Crect fill%3D"%2300acc1" width%3D"100" height%3D"100" x%3D"0" y%3D"0" %2F%3E%3Ctext x%3D"50%25" y%3D"50%25" font-family%3D"Arial%2C sans-serif" font-size%3D"50" font-weight%3D"400" fill%3D"%23ffffff" text-anchor%3D"middle" dy%3D"17.800"%3EP%3C%2Ftext%3E%3C%2Fg%3E%3C%2Fsvg%3E`,
		DateCreated: uint64(time.Now().Unix()),
		LastLogin:   uint64(time.Now().Unix()),
	}

	db.Create(&account)

	// Create the Claims (info encoded inside the token)
	claims := jwt.MapClaims{
		"id":  account.ID,
		"exp": time.Now().Add(time.Hour * 72).Unix(),
	}
	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Generate encoded token and send it as response.
	t, err := token.SignedString(privateKey)
	if err != nil {
		log.Printf("token.SignedString: %v", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.JSON(fiber.Map{"token": t, "avatar": account.Avatar, "supporter": account.Supporter, "motd": motd})
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

	username := account.Username

	// websocket.Conn bindings https://pkg.go.dev/github.com/fasthttp/websocket?tab=doc#pkg-index
	var (
		mt  int
		msg []byte
		err error
	)
	// Start a goroutine to listen for new messages
	go func() {
		for {
			recv_msg := <-BroadcastChan
			recv_processed_msg := RecievedMessageResponse{
				Cmd:     "recv_msg",
				UserId:  recv_msg.data.Username, // TODO: Replace this with the User ID
				Message: recv_msg.data.Message,
			}
			log.Println("Websocket Message")
			recv_msg_json, err := json.Marshal(recv_processed_msg)
			if err != nil {
				c.Close()
				return
			}
			if err := c.WriteMessage(websocket.TextMessage, recv_msg_json); err != nil {
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
			db_msg := Messages{Message: string(r.Message), Username: username, Channel: channel, Timestamp: uint64(time.Now().Unix())}
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
