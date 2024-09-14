package main

import (
	"gorm.io/gorm"
)

type Messages struct {
	gorm.Model
	Channel   string
	Message   string
	Username  string
	Timestamp uint64
}

type Accounts struct {
	gorm.Model
	Username     string `gorm:"uniqueIndex"`
	PasswordHash string
	Avatar       string
	Supporter    bool
	DateCreated  uint64
	LastLogin    uint64
}
