package main

import (
	"encoding/json"
	"slices"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type RankModifyJson struct {
	UserId int64
	Rank   string
}
type DeleteRankRequestJson struct {
	RemoveRankFromUsers bool
	Rank                string
}

type ResetPasswordAdmin struct {
	UserId      int64
	NewPassword string
}

func GrantRanksAPI(c *fiber.Ctx) error {
	if err := CheckIfTokenHasRank(c, "CanGrantRanks"); err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	var requestInfo RankModifyJson
	if err := json.Unmarshal(c.Body(), &requestInfo); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	userAccount := Accounts{}
	result := db.First(&userAccount, "id = ?", requestInfo.UserId)
	if result.Error != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	if result.RowsAffected == 0 {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if err := AddRankToUser(int64(userAccount.ID), requestInfo.Rank); err != nil {
		return c.SendString(err.Error())
	}
	return c.SendString("sucess!")
}

func RevokeRanksAPI(c *fiber.Ctx) error {
	if err := CheckIfTokenHasRank(c, "CanRevokeRanks"); err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	var requestInfo RankModifyJson
	if err := json.Unmarshal(c.Body(), &requestInfo); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	userAccount := Accounts{}
	result := db.First(&userAccount, "id = ?", requestInfo.UserId)
	if result.Error != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	if result.RowsAffected == 0 {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if err := RemoveRankFromUser(int64(userAccount.ID), requestInfo.Rank); err != nil {
		return c.SendString(err.Error())
	}
	return c.SendString("sucess!")
}

func CreateRankAPI(c *fiber.Ctx) error {
	// Check if the user is authorized to do this action
	if err := CheckIfTokenHasRank(c, "CanCreateRanks"); err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	var rank DefaultRanksJson
	if err := json.Unmarshal(c.Body(), &rank); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	// Check if rank exists
	existingRank := Ranks{}
	result := db.Where("rank_name = ?", rank.RankName).First(&existingRank)
	if result.RowsAffected == 0 {
		// Rank doesn't exist, create it

		// Marshal parent and subtractive ranks to JSON
		parentRanksJSON, err := json.Marshal(rank.ParentRanks)
		if err != nil {
			c.SendStatus(fiber.StatusBadRequest)
			return c.SendString("failed to marshal parent ranks: " + err.Error())
		}
		subtractiveRanksJSON, err := json.Marshal(rank.SubtractiveRanks)
		if err != nil {
			c.SendStatus(fiber.StatusBadRequest)
			return c.SendString("failed to marshal subtractive ranks: " + err.Error())
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
			c.SendStatus(fiber.StatusInternalServerError)
			return c.SendString("failed to create rank: " + err.Error())
		}
		return c.SendString("sucess!")
	} else {
		return c.SendStatus(fiber.StatusConflict)
	}
}

// TODO: move this to
func ChangePasswordAdmin(c *fiber.Ctx) error {
	// Decode the request JSON
	r := new(ResetPasswordAdmin)
	if err := json.Unmarshal(c.BodyRaw(), &r); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	// Get the user entry in the DB
	account := Accounts{}
	result := db.First(&account, "id = ?", r.UserId)
	if result.Error != nil {
		return c.SendString("account does not exist!")
	}
	if result.RowsAffected == 0 {
		return c.SendString("account does not exist!")
	}

	// Check permissions
	ranks, err := GetEffectivePermissions(account.Ranks)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if !slices.Contains(ranks, "CanResetOtherUsersPasswords") {
		return c.SendString("changing profile pictures is restricted!")
	}

	// Generate New password Password
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

// func ModifyRankAPI(c *fiber.Ctx) error {
// 	// Check if the user is authorized to do this action
// 	if err := CheckIfTokenHasRank(c, "CanModifyRanks"); err != nil {
// 		return c.SendStatus(fiber.StatusUnauthorized)
// 	}

// 	var rank DefaultRanksJson
// 	if err := json.Unmarshal(c.Body(), &rank); err != nil {
// 		return c.SendStatus(fiber.StatusBadRequest)
// 	}

// 	// Check if rank exists
// 	existingRank := Ranks{}
// 	result := db.Where("rank_name = ?", rank.RankName).First(&existingRank)
// 	if result.RowsAffected != 0 {
// 		// Rank exist, let's modify it

// 		// Check if the marshal elements were included
// 		// and if it is, we set that to the rank.
// 		if rank.ParentRanks != nil {
// 			parentRanksJSON, err := json.Marshal(rank.ParentRanks)
// 			if err != nil {
// 				c.SendStatus(fiber.StatusBadRequest)
// 				return c.SendString("failed to marshal parent ranks: " + err.Error())
// 			}
// 			existingRank.ParentRanks = string(parentRanksJSON)
// 		}
// 		if rank.SubtractiveRanks != nil {
// 			subtractiveRanksJSON, err := json.Marshal(rank.SubtractiveRanks)
// 			if err != nil {
// 				c.SendStatus(fiber.StatusBadRequest)
// 				return c.SendString("failed to marshal subtractive ranks: " + err.Error())
// 			}
// 			existingRank.SubtractiveRanks = string(subtractiveRanksJSON)
// 		}

// 		if &rank.RankStrength != nil {
// 			existingRank.RankStrength = rank.RankStrength
// 		}
// 		if &rank.RankName != nil {
// 			existingRank.RankName = rank.RankName
// 		}
// 		if &rank.Color != nil {
// 			existingRank.Color = rank.Color
// 		}
// 		if &rank.ShowToOtherUsers != nil {
// 			existingRank.ShowToOtherUsers = rank.ShowToOtherUsers
// 		}

// 		if err := db.Save(&existingRank).Error; err != nil {
// 			c.SendStatus(fiber.StatusInternalServerError)
// 			return c.SendString("failed to modify rank: " + err.Error())
// 		}
// 		return c.SendString("sucess!")
// 	} else {
// 		return c.SendStatus(fiber.StatusBadRequest)
// 	}
// }

func DeleteRankAPI(c *fiber.Ctx) error {
	// Check if the user is authorized to do this action
	if err := CheckIfTokenHasRank(c, "CanDeleteRanks"); err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	var rank DefaultRanksJson
	if err := json.Unmarshal(c.Body(), &rank); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	// Check if rank exists
	existingRank := Ranks{}
	result := db.Where("rank_name = ?", rank.RankName).First(&existingRank)
	if result.RowsAffected != 0 {
		// Rank exists, time to remove it!
		if err := result.Delete(&existingRank).Error; err != nil {
			return c.SendString("failed to delete rank" + err.Error())
		}
		return c.SendString("sucess!")
	} else {
		return c.SendString("rank doesn't exist!")
	}
}
