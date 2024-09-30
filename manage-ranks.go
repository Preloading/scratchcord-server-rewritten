package main

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
)

type RankModifyJson struct {
	UserId int64
	Rank   string
}
type DeleteRankRequestJson struct {
	RemoveRankFromUsers bool
	Rank                string
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
