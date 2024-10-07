package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type Ranks struct {
	RankStrength     uint   `gorm:"primaryKey"`
	RankName         string `gorm:"uniqueIndex"`
	Color            string // Default: default
	ShowToOtherUsers bool
	ParentRanks      string `gorm:"type:text"` // Stored as JSON array
	SubtractiveRanks string `gorm:"type:text"` // Stored as JSON array
}
type DefaultRanksJson struct {
	RankStrength     uint
	RankName         string
	Color            string // Default: default
	ShowToOtherUsers bool
	ParentRanks      []string
	SubtractiveRanks []string
}

func (r *Ranks) GetParentRanks() ([]string, error) {
	var parentRanks []string
	err := json.Unmarshal([]byte(r.ParentRanks), &parentRanks)
	return parentRanks, err
}

func (r *Ranks) GetSubtractiveRanks() ([]string, error) {
	var subtractiveRanks []string
	err := json.Unmarshal([]byte(r.SubtractiveRanks), &subtractiveRanks)
	return subtractiveRanks, err
}

func (r *Ranks) SetParentRanks(parentRanks []string) error {
	jsonRanks, err := json.Marshal(parentRanks)
	if err != nil {
		return err
	}
	r.ParentRanks = string(jsonRanks)
	return nil
}

func (r *Ranks) SetSubtractiveRanks(subtractiveRanks []string) error {
	jsonRanks, err := json.Marshal(subtractiveRanks)
	if err != nil {
		return err
	}
	r.SubtractiveRanks = string(jsonRanks)
	return nil
}

func AddRankToUser(id int64, rankToAdd string) error {
	var user Accounts
	db.First(&user, "id = ?", id)
	var ranks []string
	err := json.Unmarshal([]byte(user.Ranks), &ranks)
	if err != nil {
		return err
	}
	if slices.Contains(ranks, rankToAdd) {
		return errors.New("rank already exists")
	}
	ranks = append(ranks, rankToAdd)
	jsonRanks, err := json.Marshal(ranks)
	if err != nil {
		return err
	}
	if err := db.Model(&user).Where("id = ?", id).Update("ranks", jsonRanks).Error; err != nil {
		return err
	}

	return nil
}

func RemoveRankFromUser(id int64, rankToRemove string) error {
	var user Accounts
	db.First(&user, "id = ?", id)
	var ranks []string
	err := json.Unmarshal([]byte(user.Ranks), &ranks)
	if err != nil {
		return err
	}
	if !slices.Contains(ranks, rankToRemove) {
		return errors.New("user doesn't have the rank attempting to be removed")
	}

	// Find the index of the rank to remove
	indexToRemove := -1
	for i, r := range ranks {
		if r == rankToRemove {
			indexToRemove = i
			break
		}
	}

	// If the rank was found, remove it
	if indexToRemove != -1 {
		ranks = append(ranks[:indexToRemove], ranks[indexToRemove+1:]...)

		jsonRanks, err := json.Marshal(ranks)
		if err != nil {
			return err
		}
		if err := db.Model(&user).Where("id = ?", id).Update("ranks", jsonRanks).Error; err != nil {
			return err
		}
	}

	return nil
}

func InitializeRanksFromJSON(data []byte) error {
	// Parse the JSON data
	var defaultRanks []DefaultRanksJson
	err := json.Unmarshal(data, &defaultRanks)
	if err != nil {
		return fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Insert default ranks into the database
	for _, rank := range defaultRanks {
		// Check if rank already exists
		existingRank := Ranks{}
		result := db.Where("rank_name = ?", rank.RankName).First(&existingRank)
		if result.RowsAffected == 0 {
			// Rank doesn't exist, create it

			// Marshal parent and subtractive ranks to JSON
			parentRanksJSON, err := json.Marshal(rank.ParentRanks)
			if err != nil {
				return fmt.Errorf("failed to marshal parent ranks: %w", err)
			}
			subtractiveRanksJSON, err := json.Marshal(rank.SubtractiveRanks)
			if err != nil {
				return fmt.Errorf("failed to marshal subtractive ranks: %w", err)
			}

			newRank := Ranks{
				RankStrength:     rank.RankStrength,
				RankName:         rank.RankName,
				Color:            rank.Color,
				ShowToOtherUsers: rank.ShowToOtherUsers,
				ParentRanks:      string(parentRanksJSON),
				SubtractiveRanks: string(subtractiveRanksJSON),
			}
			if err := db.Create(&newRank).Error; err != nil {
				return fmt.Errorf("failed to create rank: %w", err)
			}
		}
	}

	return nil
}

func GetEffectivePermissions(userInput interface{}) ([]string, error) {
	// 1. Determine input type and convert to []string
	var userRanks []string
	switch v := userInput.(type) {
	case []string:
		userRanks = v
	case string:
		err := json.Unmarshal([]byte(v), &userRanks)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON string into []string: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid input type: expected []string or string (JSON array), got %T", userInput)
	}

	// 2. Create a map to store all permissions (including inherited)
	allPermissions := make(map[string]bool)

	// 3. Helper function to recursively add parent rank permissions
	var addParentPermissions func(rankName string) error
	addParentPermissions = func(rankName string) error {
		// 4. Fetch the rank details from the database
		var rank Ranks
		if err := db.Where("rank_name = ?", rankName).First(&rank).Error; err != nil {
			return fmt.Errorf("failed to fetch rank details: %w", err)
		}

		// 5. Add current rank's permissions
		allPermissions[rankName] = true

		// 6. Recursively add parent rank's permissions
		parentRanks, err := rank.GetParentRanks()
		if err != nil {
			return fmt.Errorf("failed to get parent ranks: %w", err)
		}
		for _, parentRank := range parentRanks {
			if err := addParentPermissions(parentRank); err != nil {
				return err
			}
		}
		return nil
	}

	// 7. Iterate through user's ranks and add permissions (including parents)
	for _, rankName := range userRanks {
		if err := addParentPermissions(rankName); err != nil {
			return nil, err
		}
	}

	// 8. Iterate through user's ranks again to process subtractive ranks
	for i := len(userRanks) - 1; i >= 0; i-- {
		rankName := userRanks[i]

		// 9. Fetch the rank details from the database
		var rank Ranks
		if err := db.Where("rank_name = ?", rankName).First(&rank).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch rank details: %w", err)
		}

		// 10. Remove subtractive ranks and their sub-ranks
		subtractiveRanks, err := rank.GetSubtractiveRanks()
		if err != nil {
			return nil, fmt.Errorf("failed to get subtractive ranks: %w", err)
		}
		for _, subtractiveRank := range subtractiveRanks {
			delete(allPermissions, subtractiveRank)
		}
	}

	// 11. Convert the map keys (which are the effective permissions) to a slice
	effectivePermissions := make([]string, 0, len(allPermissions))
	for permission := range allPermissions {
		effectivePermissions = append(effectivePermissions, permission)
	}

	return effectivePermissions, nil
}

func GetRankInfo(c *fiber.Ctx) error {
	rankName := c.Query("rankname")
	if rankName == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	var rank Ranks
	if err := db.Where("rank_name = ?", rankName).First(&rank).Error; err != nil {
		return c.SendString("rank doesn't exist!")
	}
	return c.JSON(rank)
}

//go:embed config/default_ranks.json
var defaultRanksJson []byte

//go:embed config/required_ranks.json
var requiredRanksJson []byte

func InitializeRanks() {
	var testRank Ranks
	result := db.First(&testRank)
	if result.RowsAffected == 0 {
		fmt.Println("Ranks not found! Restoring default values.")
		err := InitializeRanksFromJSON(defaultRanksJson)
		if err != nil {
			fmt.Println("Error restoring default ranks:", err)
			os.Exit(1)
		}
	}

	// Initialize required ranks from JSON file
	err := InitializeRanksFromJSON(requiredRanksJson)
	if err != nil {
		fmt.Println("Error veriafying & readding required ranks:", err)
		os.Exit(1)
	}
}

func CheckIfTokenHasRank(c *fiber.Ctx, rank string) error {

	// Check if the account being used to make this request has the permission
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	accountId := claims["id"].(float64)

	if check_if_token_expired(user) {
		return errors.New("token expired")
	}

	adminAccount := Accounts{}
	result := db.First(&adminAccount, "id = ?", accountId)
	if result.Error != nil {
		return errors.New("user does not exist")
	}
	if result.RowsAffected == 0 {
		return errors.New("user does not exist")
	}

	ranks, err := GetEffectivePermissions(adminAccount.Ranks)
	if err != nil {
		return errors.New("an internal server error occured")
	}
	if !slices.Contains(ranks, rank) {
		return errors.New("user doess not contain the rank")
	}
	return nil
}
