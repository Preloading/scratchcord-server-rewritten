package main

import (
	"gorm.io/gorm"
)

type Messages struct {
	gorm.Model
	Channel string
	Message string

	// Type... types
	// 1 - Normal Message
	// 2 - Nudge
	// 3 - Typing, does not store in DB
	// 4 - Game Start Request
	// 100 - Global Message (admin only)
	// 101 - Global TTS Message (admin only)
	// 102 - Channel Special Message (admin only)
	// 103 - Channel Special TTS Message (admin only)
	// 104 - Kick User (admin only)
	// 105 - Kick All Users (admin only)
	Type uint8

	UserId    uint
	Timestamp uint64
}

type Accounts struct {
	gorm.Model
	Username     string `gorm:"uniqueIndex"`
	PasswordHash string
	Avatar       string
	DateCreated  uint64
	LastLogin    uint64
	Ranks        string `gorm:"type:text"` // Stored as JSON array
}
