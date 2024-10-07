package main

import (
	"fmt"
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
	allowedTypes := []string{"image/jpeg", "image/png"}
	if !slices.Contains(allowedTypes, file.Header.Get("Content-Type")) {
		return c.SendString("file type not supported!")
	}
	destination := fmt.Sprintf("./uploads/profile-pictures/%s", uuid.New())
	if err := c.SaveFile(file, destination); err != nil {
		// Handle error
		return err
	}
	return c.SendString("sucess!")
}
