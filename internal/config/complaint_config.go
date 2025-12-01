package config

import "time"

const (
	// Reputation
	InitialReputation      = 1000
	MaxReputation          = 1000
	MinReputation          = 0
	SuccessfulDialogReward = 1
	EarlyDisconnectPenalty = -1
	ConfirmedComplaintBonus = 50
	ReputationRecoveryAmount = 100

	// Ban
	BanThresholdReputation = 500
	BanThresholdFrequency = 5
	BanFrequencyWindow     = 24 * time.Hour
	BanLevel1Duration      = 30 * time.Minute
	BanLevel2Duration      = 6 * time.Hour
	BanLevel3Duration      = 24 * time.Hour

	// Dialog
	SuccessfulDialogDuration = 10 * time.Minute
	SuccessfulDialogMessages = 10
	EarlyDisconnectDuration  = 2 * time.Minute
	EarlyDisconnectMessages  = 2
)

var ComplaintWeights = map[string]int{
	"Low":      5,
	"Medium":   50,
	"Critical": 250,
}
