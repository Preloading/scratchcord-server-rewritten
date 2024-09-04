package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"log"
	"runtime/debug"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/golang-jwt/jwt"

	// Auth

	jwtware "github.com/gofiber/contrib/jwt"
)

type LoginRequest struct {
	Username string
	Password string
}

var (
	// Obviously, this is just a test example. Do not do this in production.
	// In production, you would have the private key and public key pair generated
	// in advance. NEVER add a private key to any GitHub repo.
	privateKey *rsa.PrivateKey
)

func main() {
	// Configure runtime settings
	debug.SetGCPercent(35) // 35% limit for GC

	// MOTD
	motd := "Welcome to scratchcord!"

	// Auth setup
	rng := rand.Reader
	var err error
	privateKey, err = rsa.GenerateKey(rng, 2048)
	if err != nil {
		log.Fatalf("rsa.GenerateKey: %v", err)
	}

	// Create fiber application
	app := fiber.New()

	// app.Use(cors.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Accept-Language, Content-Length",
	}))
	app.Get("/hello", func(c *fiber.Ctx) error {
		// Variable is only valid within this handler
		return c.SendString("Hello, World!")
	})

	app.Post("/login", func(c *fiber.Ctx) error {
		r := new(LoginRequest)

		if err := json.Unmarshal(c.BodyRaw(), &r); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Create the Claims
		claims := jwt.MapClaims{
			"username": r.Username,
			"exp":      time.Now().Add(time.Hour * 72).Unix(),
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
	})
	// JWT Middleware
	app.Use(jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{
			JWTAlg: jwtware.RS256,
			Key:    privateKey.Public(),
		},
	}))

	app.Get("/check_auth", func(c *fiber.Ctx) error {
		user := c.Locals("user").(*jwt.Token)
		claims := user.Claims.(jwt.MapClaims)
		name := claims["name"].(string)
		return c.SendString("Welcome " + name)
	})

	// Add a websocket path
	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			// c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// Bind CloudLink server to websocket path
	app.Get("/ws/:id", websocket.New(func(c *websocket.Conn) {
		// c.Locals is added to the *websocket.Conn
		log.Println(c.Locals("allowed"))  // true
		log.Println(c.Params("id"))       // 123
		log.Println(c.Query("v"))         // 1.0
		log.Println(c.Cookies("session")) // ""

		// websocket.Conn bindings https://pkg.go.dev/github.com/fasthttp/websocket?tab=doc#pkg-index
		var (
			mt  int
			msg []byte
			err error
		)
		for {
			if mt, msg, err = c.ReadMessage(); err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", msg)

			if err = c.WriteMessage(mt, msg); err != nil {
				log.Println("write:", err)
				break
			}
			if err = c.WriteMessage(mt, []byte("Hello this is websockets!")); err != nil {
				log.Println("write:", err)
				break
			}
		}

	}))

	app.Get("/monitor", monitor.New(
		monitor.Config{
			Title:   "Metrics",
			Refresh: (50 * time.Millisecond),
		},
	))

	log.Fatal(app.Listen(":3000"))
	// Access the websocket server: ws://0.0.0.0:3000/

	//log.Fatal(app.ListenTLS("0.0.0.0:3000", "./cert.pem", "./key.pem"))
	// Access the websocket server: wss://0.0.0.0:3000/
}
