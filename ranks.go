package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
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

func GetEffectivePermissions(userRanks []string) ([]string, error) {
	// 1. Create a map to store all permissions (including inherited)
	allPermissions := make(map[string]bool)

	// 2. Iterate through user's ranks from highest to lowest strength
	for i := len(userRanks) - 1; i >= 0; i-- {
		rankName := userRanks[i]

		// 3. Fetch the rank details from the database
		var rank Ranks
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
		var rank Ranks
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
		fmt.Println("Error verifying & readding required ranks:", err)
		os.Exit(1)
	}
}

func wmain() {
	// Example usage:
	// Assuming you have a database connection established

	// Example user with ranks
	userRanks := []string{"Supporter", "Member"}

	// Get effective permissions
	effectivePermissions, err := GetEffectivePermissions(userRanks)
	if err != nil {
		fmt.Println("Error getting effective permissions:", err)
		os.Exit(1)
	}

	// Print effective permissions
	fmt.Println("Effective Permissions:", effectivePermissions)
}
