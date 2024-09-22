package main

type Rank struct {
	RankStrength     uint
	RankName         string
	Color            string // Default: default
	ShowToOtherUsers bool
	ParentRanks      []string
	SubtractiveRanks []string
}
