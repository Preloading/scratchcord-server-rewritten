package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Rank struct {
	RankStrength     uint   `gorm:"primaryKey"`
	RankName         string `gorm:"uniqueIndex"`
	Color            string // Default: default
	ShowToOtherUsers bool
	ParentRanks      string `gorm:"type:text"` // Stored as JSON array
	SubtractiveRanks string `gorm:"type:text"` // Stored as JSON array
}

func (r *Rank) GetParentRanks() ([]string, error) {
	var parentRanks []string
	err := json.Unmarshal([]byte(r.ParentRanks), &parentRanks)
	return parentRanks, err
}

func (r *Rank) GetSubtractiveRanks() ([]string, error) {
	var subtractiveRanks []string
	err := json.Unmarshal([]byte(r.SubtractiveRanks), &subtractiveRanks)
	return subtractiveRanks, err
}

func (r *Rank) SetParentRanks(parentRanks []string) error {
	jsonRanks, err := json.Marshal(parentRanks)
	if err != nil {
		return err
	}
	r.ParentRanks = string(jsonRanks)
	return nil
}

func (r *Rank) SetSubtractiveRanks(subtractiveRanks []string) error {
	jsonRanks, err := json.Marshal(subtractiveRanks)
	if err != nil {
		return err
	}
	r.SubtractiveRanks = string(jsonRanks)
	return nil
}

func InitializeRanksFromJSON(filePath string) error {
	// Read the JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Parse the JSON data
	var defaultRanks []Rank
	err = json.Unmarshal(data, &defaultRanks)
	if err != nil {
		return fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Insert default ranks into the database
	for _, rank := range defaultRanks {
		// Check if rank already exists
		existingRank := Rank{}
		result := db.Where("rank_name = ?", rank.RankName).First(&existingRank)
		if result.RowsAffected == 0 {
			// Rank doesn't exist, create it
			if err := db.Create(&rank).Error; err != nil {
				return fmt.Errorf("failed to create rank: %w", err)
			}
		}
	}

	return nil
}

func GetEffectivePermissions(userRanks []string) ([]string, error) {
	// 1. Create a map to store all permissions (including inherited)
	allPermissions := make(map[string]bool)

	// 2. Iterate through user's ranks from highest to lowest strength
	for i := len(userRanks) - 1; i >= 0; i-- {
		rankName := userRanks[i]

		// 3. Fetch the rank details from the database
		var rank Rank
		if err := db.Where("rank_name = ?", rankName).First(&rank).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch rank details: %w", err)
		}

		// 4. Add current rank's permissions
		allPermissions[rankName] = true

		// 5. Add parent rank's permissions
		parentRanks, err := rank.GetParentRanks()
		if err != nil {
			return nil, fmt.Errorf("failed to get parent ranks: %w", err)
		}
		for _, parentRank := range parentRanks {
			allPermissions[parentRank] = true
		}
	}

	// 6. Iterate through user's ranks again to process subtractive ranks
	for i := len(userRanks) - 1; i >= 0; i-- {
		rankName := userRanks[i]

		// 7. Fetch the rank details from the database
		var rank Rank
		if err := db.Where("rank_name = ?", rankName).First(&rank).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch rank details: %w", err)
		}

		// 8. Remove subtractive ranks and their sub-ranks
		subtractiveRanks, err := rank.GetSubtractiveRanks()
		if err != nil {
			return nil, fmt.Errorf("failed to get subtractive ranks: %w", err)
		}
		for _, subtractiveRank := range subtractiveRanks {
			delete(allPermissions, subtractiveRank)
		}
	}

	// 9. Convert the map keys (which are the effective permissions) to a slice
	effectivePermissions := make([]string, 0, len(allPermissions))
	for permission := range allPermissions {
		effectivePermissions = append(effectivePermissions, permission)
	}

	return effectivePermissions, nil
}

func wmain() {
	// Example usage:
	// Assuming you have a database connection established

	// Initialize ranks from JSON file
	err := InitializeRanksFromJSON("config/ranks.json")
	if err != nil {
		fmt.Println("Error initializing ranks:", err)
		os.Exit(1)
	}

	// Example user with ranks
	userRanks := []string{"Moderator", "VIP"}

	// Get effective permissions
	effectivePermissions, err := GetEffectivePermissions(userRanks)
	if err != nil {
		fmt.Println("Error getting effective permissions:", err)
		os.Exit(1)
	}

	// Print effective permissions
	fmt.Println("Effective Permissions:", effectivePermissions)
}
