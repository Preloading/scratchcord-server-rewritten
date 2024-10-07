package main

import (
	"fmt"
	"mime"
	"slices"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func UploadFile(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		// Handle error
		return err
	}
	if file.Size > 4*1024*1024 {
		c.SendString("file too large!")
	}
	allowedTypes := []string{"image/jpeg", "image/jpg", "image/png", "image/webp", "image/gif"}
	contentType := file.Header.Get("Content-Type")
	if !slices.Contains(allowedTypes, contentType) {
		return c.SendString("file type not supported!")
	}

	fileExt, _ := mime.ExtensionsByType(contentType) // If this errors, something has gone terribly wrong, and one of our 4 image types is bad.

	if fileExt[0] == ".jfif" { // Personal gripe with MIME. I'd like it to output .jpg as default cuz what the fuck is a .jfif
		fileExt[0] = ".jpg"
	}

	fileName := uuid.NewString() + fileExt[0]
	destination := fmt.Sprintf("./uploads/profile-pictures/%s", fileName)
	if err := c.SaveFile(file, destination); err != nil {
		// Handle error
		return err
	}
	return c.SendString("sucess!")
}
