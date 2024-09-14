package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username string
	Password string
}

type RegisterRequest struct { // This is the same for now, but maybe later more info will be added, like email? idk. Least it'll have the option
	Username string
	Password string
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
