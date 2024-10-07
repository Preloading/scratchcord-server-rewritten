package main

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"slices"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/chai2010/webp"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nfnt/resize"
)

func UploadProfilePicture(c *fiber.Ctx) error {
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

	// Temporarly removed as rank is not added yet

	// ranks, err := GetEffectivePermissions(account.Ranks)
	// if err != nil {
	// 	return c.SendStatus(fiber.StatusInternalServerError)
	// }

	// if !slices.Contains(ranks, "CanChangeProfilePicture") {
	// 	return c.SendString("changing profile pictures is restricted!")
	// }

	// Do image processing
	file, err := c.FormFile("file")
	if err != nil {
		return err
	}

	if file.Size > 3*1024*1024 { // 3MB max
		return c.SendString("file too large!")
	}

	allowedTypes := []string{"image/jpeg", "image/jpg", "image/png", "image/webp", "image/gif"}
	contentType := file.Header.Get("Content-Type")
	if !slices.Contains(allowedTypes, contentType) {
		return c.SendString("file type not supported!")
	}

	// Open the uploaded file
	fileData, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer fileData.Close()
	// Decode the image
	img, _, err := image.Decode(fileData)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize and crop the image to 512x512
	img = resizeAndCrop(img, 512, 512)

	// Encode the image to WebP *before* saving
	var buf bytes.Buffer // Use an in-memory buffer
	err = webp.Encode(&buf, img, &webp.Options{Quality: 80})
	if err != nil {
		return fmt.Errorf("failed to encode image to WebP: %w", err)
	}

	// Save the WebP image data to a file
	fileName := uuid.NewString() + ".webp"
	destination := fmt.Sprintf("./uploads/profile-pictures/%s", fileName)
	err = os.WriteFile(destination, buf.Bytes(), 0644) // Save from buffer
	if err != nil {
		return fmt.Errorf("failed to save WebP image: %w", err)
	}
	account.Avatar = server_url + "/uploads/profile-pictures/" + fileName
	db.Save(&account)
	return c.SendString("success!")
}

// resizeAndCrop resizes and crops the image to the specified dimensions,
// maintaining a 1:1 aspect ratio.
func resizeAndCrop(img image.Image, width, height int) image.Image {
	// Resize the image to fit within the specified dimensions
	resizedImg := resize.Thumbnail(uint(width), uint(height), img, resize.Lanczos3)

	// Calculate cropping parameters
	resizedWidth := resizedImg.Bounds().Dx()
	resizedHeight := resizedImg.Bounds().Dy()
	var x, y, cropWidth, cropHeight int
	if resizedWidth > resizedHeight {
		cropWidth = resizedHeight
		cropHeight = resizedHeight
		x = (resizedWidth - cropWidth) / 2
		y = 0
	} else {
		cropWidth = resizedWidth
		cropHeight = resizedWidth
		x = 0
		y = (resizedHeight - cropHeight) / 2
	}

	// Crop the image
	croppedImg := image.NewRGBA(image.Rect(0, 0, cropWidth, cropHeight))
	for i := 0; i < cropWidth; i++ {
		for j := 0; j < cropHeight; j++ {
			croppedImg.Set(i, j, resizedImg.At(x+i, y+j))
		}
	}

	return croppedImg
}
