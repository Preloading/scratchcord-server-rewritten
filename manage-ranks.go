package main

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
)

type RankModifyJson struct {
	UserId int64
	Rank   string
}

func GrantRanksAPI(c *fiber.Ctx) error {
	if err := CheckIfTokenHasRank(c, "CanGrantRanks"); err != nil {
		return c.SendStatus(fiber.StatusForbidden)
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
		return c.SendStatus(fiber.StatusForbidden)
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
