package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username      string
	Password      string
	ClientVersion string
}

type ReauthRequest struct {
	ClientVersion string
}

type RegisterRequest struct { // This is the same for now, but maybe later more info will be added, like email? idk. Least it'll have the option
	Username      string
	Password      string
	ClientVersion string
}

type ResetPassword struct {
	OldPassword string
	NewPassword string
}

func reauth(c *fiber.Ctx) error {
	// Verify Client is compatible
	r := new(ReauthRequest)

	if err := json.Unmarshal(c.BodyRaw(), &r); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	// Check to see if the current token is valid.
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	var accountId string
	if id, ok := claims["id"].(float64); ok {
		accountId = fmt.Sprintf("%f", id)
	} else {
		// Handle the case where "id" is not a float64
		return c.SendStatus(fiber.StatusInternalServerError) // Or appropriate error
	}

	if check_if_token_expired(user) {
		return c.SendString("token expired!")
	}

	account := Accounts{}
	result := db.First(&account, "id = ?", accountId)
	if result.Error != nil {
		return c.SendString("account does not exist!")
	}
	if result.RowsAffected == 0 {
		return c.SendString("account does not exist!")
	}

	ranks, err := GetEffectivePermissions(account.Ranks)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if !slices.Contains(ranks, "CanBeLoggedInto") {
		return c.SendString("account login is restricted!")
	}

	// Create the Claims (info encoded inside the token)
	newClaims := jwt.MapClaims{
		"id":  account.ID,
		"exp": time.Now().Add(time.Hour * 72).Unix(),
	}
	// Create new token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, newClaims)

	// Generate encoded token and send it as response.
	t, err := token.SignedString(privateKey)
	if err != nil {
		log.Printf("token.SignedString: %v", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	// update last login
	account.LastLogin = uint64(time.Now().Unix())
	db.Save(&account)
	return c.JSON(fiber.Map{"token": t, "avatar": account.Avatar, "ranks": ranks, "motd": motd})
}

func login(c *fiber.Ctx) error {
	// Decode login params
	r := new(LoginRequest)
	if err := json.Unmarshal(c.BodyRaw(), &r); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	// Check if the client is supported
	if !slices.Contains(permitted_protocol_versions, r.ClientVersion) {
		return c.SendString("client version not supported!")
	}

	// Get account from db
	account := Accounts{}
	result := db.First(&account, "username = ?", r.Username)
	if result.Error != nil {
		return c.SendString("account does not exist!")
	}
	if result.RowsAffected == 0 {
		return c.SendString("account does not exist!")
	}

	// Check if the account is allowed to sign in
	ranks, err := GetEffectivePermissions(account.Ranks)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if !slices.Contains(ranks, "CanBeLoggedInto") {
		return c.SendString("account login is restricted!")
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
	return c.JSON(fiber.Map{"token": t, "avatar": account.Avatar, "ranks": ranks, "motd": motd})
}

func register(c *fiber.Ctx) error {
	r := new(RegisterRequest)

	if err := json.Unmarshal(c.BodyRaw(), &r); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	// Check if client is supported
	if !slices.Contains(permitted_protocol_versions, r.ClientVersion) {
		return c.SendString("client version not supported!")
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

	// Generate the default avatar
	fileName := uuid.NewString() + ".webp"
	destination := fmt.Sprintf("./uploads/profile-pictures/%s", fileName)
	out, err := os.Create(destination)
	if err != nil {
		panic(err) // This should NEVER fail unless we run out of disk space.
	}

	// We use an API to create this
	// Example of a request we make to generate avatars: https://api.dicebear.com/9.x/initials/webp?chars=1&seed=Preloading
	requestURL := fmt.Sprintf("https://api.dicebear.com/9.x/initials/webp?chars=1&seed=%s", url.QueryEscape(r.Username))
	res, err := http.Get(requestURL)
	if err != nil {
		fmt.Printf("error making http request to dicebear: %s\n", err)
		// Since I do not have any fallback avatar, I essentially need to error it.
		c.SendString("An internal error occured while generating your avatar! Try reregistering. If that does not work it, please email webmaster@loganserver.net")
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	// Now that we have the image, we should store it
	// Save the WebP image data to a file
	defer out.Close()
	io.Copy(out, res.Body)

	account := Accounts{
		Username:     r.Username,
		PasswordHash: string(hash),
		Avatar:       server_url + "/uploads/profile-pictures/" + fileName,
		DateCreated:  uint64(time.Now().Unix()),
		LastLogin:    uint64(time.Now().Unix()),
		Ranks:        `["Member"]`,
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
	ranks, err := GetEffectivePermissions(account.Ranks)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.JSON(fiber.Map{"token": t, "avatar": account.Avatar, "ranks": ranks, "motd": motd})
}

func change_password(c *fiber.Ctx) error {
	// Decode the user token and get the user entry in the DB
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	var accountId string
	if id, ok := claims["id"].(float64); ok {
		accountId = fmt.Sprintf("%f", id)
	} else {
		// Handle the case where "id" is not a float64
		return c.SendStatus(fiber.StatusInternalServerError) // Or appropriate error
	}

	if check_if_token_expired(user) {
		return c.SendString("token expired!")
	}

	account := Accounts{}
	result := db.First(&account, "id = ?", accountId)
	if result.Error != nil {
		return c.SendString("account does not exist!")
	}
	if result.RowsAffected == 0 {
		return c.SendString("account does not exist!")
	}

	// Check if they can change their password
	ranks, err := GetEffectivePermissions(account.Ranks)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if !slices.Contains(ranks, "CanChangePassword") {
		return c.SendString("changing profile pictures is restricted!")
	}

	// Decode the request JSON
	r := new(ResetPassword)
	if err := json.Unmarshal(c.BodyRaw(), &r); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	// Check if the old password matches the current password
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(r.OldPassword)); err != nil {
		return c.SendString("wrong password!")
	}

	// Generate New Password
	hash, err := bcrypt.GenerateFromPassword([]byte(r.NewPassword), hash_default_cost)
	if err != nil {
		return c.SendString("password invalid!")
	}

	// Save the new password in the DB
	account.PasswordHash = string(hash)
	db.Save(&account)

	// TODO: Make it invalidate all past tokens
	return c.SendString("sucess!")
}

func get_user_info(c *fiber.Ctx) error {
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
	effectiveRanks, err := GetEffectivePermissions(user.Ranks)
	if err != nil {
		c.SendString("user ranks is corrupted! please contact administrator!")
	}
	userResponse := UserInfoResponse{
		ID:             user.ID,
		Username:       user.Username,
		Avatar:         user.Avatar,
		Ranks:          user.Ranks,
		EffectiveRanks: effectiveRanks,
		DateCreated:    user.DateCreated,
		LastLogin:      user.LastLogin,
	}
	return c.JSON(userResponse)
}

func check_auth(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	name := claims["username"].(string)
	return c.SendString("Welcome " + name)
}

func check_if_token_expired(token *jwt.Token) bool {
	claims := token.Claims.(jwt.MapClaims)
	exp := claims["exp"].(float64)
	if i := int64(exp); i < time.Now().Unix() {
		return true
	} else {
		return false
	}
}

func register_default_admin_account() {

	// Generate Password in BCrypt form
	hash, err := bcrypt.GenerateFromPassword([]byte(admin_password), hash_default_cost)
	if err != nil {
		panic("Admin password is invalid!")
	}

	// Check if username is already claimed.
	var count int64 = 0
	result := db.Model(&Accounts{}).Where("username = ?", "Administrator")
	if result.Count(&count); count > 0 {
		result.Update("PasswordHash", string(hash))
		result.Update("Ranks", `["Administrator"]`)
	} else {
		account := Accounts{
			Username:     "Administrator",
			PasswordHash: string(hash),
			// Temp avatar till i get proper storage running
			Avatar:      `data:image/svg+xml;utf8,%3Csvg xmlns%3D"http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg" viewBox%3D"0 0 100 100"%3E%3Cmetadata xmlns%3Ardf%3D"http%3A%2F%2Fwww.w3.org%2F1999%2F02%2F22-rdf-syntax-ns%23" xmlns%3Axsi%3D"http%3A%2F%2Fwww.w3.org%2F2001%2FXMLSchema-instance" xmlns%3Adc%3D"http%3A%2F%2Fpurl.org%2Fdc%2Felements%2F1.1%2F" xmlns%3Adcterms%3D"http%3A%2F%2Fpurl.org%2Fdc%2Fterms%2F"%3E%3Crdf%3ARDF%3E%3Crdf%3ADescription%3E%3Cdc%3Atitle%3EInitials%3C%2Fdc%3Atitle%3E%3Cdc%3Acreator%3EDiceBear%3C%2Fdc%3Acreator%3E%3Cdc%3Asource xsi%3Atype%3D"dcterms%3AURI"%3Ehttps%3A%2F%2Fgithub.com%2Fdicebear%2Fdicebear%3C%2Fdc%3Asource%3E%3Cdcterms%3Alicense xsi%3Atype%3D"dcterms%3AURI"%3Ehttps%3A%2F%2Fcreativecommons.org%2Fpublicdomain%2Fzero%2F1.0%2F%3C%2Fdcterms%3Alicense%3E%3Cdc%3Arights%3E%E2%80%9EInitials%E2%80%9D (https%3A%2F%2Fgithub.com%2Fdicebear%2Fdicebear) by %E2%80%9EDiceBear%E2%80%9D%2C licensed under %E2%80%9ECC0 1.0%E2%80%9D (https%3A%2F%2Fcreativecommons.org%2Fpublicdomain%2Fzero%2F1.0%2F)%3C%2Fdc%3Arights%3E%3C%2Frdf%3ADescription%3E%3C%2Frdf%3ARDF%3E%3C%2Fmetadata%3E%3Cmask id%3D"b0esx9i5"%3E%3Crect width%3D"100" height%3D"100" rx%3D"0" ry%3D"0" x%3D"0" y%3D"0" fill%3D"%23fff" %2F%3E%3C%2Fmask%3E%3Cg mask%3D"url(%23b0esx9i5)"%3E%3Crect fill%3D"%2300acc1" width%3D"100" height%3D"100" x%3D"0" y%3D"0" %2F%3E%3Ctext x%3D"50%25" y%3D"50%25" font-family%3D"Arial%2C sans-serif" font-size%3D"50" font-weight%3D"400" fill%3D"%23ffffff" text-anchor%3D"middle" dy%3D"17.800"%3EP%3C%2Ftext%3E%3C%2Fg%3E%3C%2Fsvg%3E`,
			DateCreated: uint64(time.Now().Unix()),
			LastLogin:   uint64(time.Now().Unix()),
			Ranks:       `["Administrator"]`,
		}

		db.Create(&account)
	}

}
