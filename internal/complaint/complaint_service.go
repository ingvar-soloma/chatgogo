// Package complaint provides the core logic for handling user complaints,
// including reputation management and applying restrictions.
package complaint

import (
	"chatgogo/backend/internal/analysis"
	"chatgogo/backend/internal/config"
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"time"
)

// Service handles the business logic for complaints.
type Service struct {
	Storage storage.Storage
}

// NewService creates a new complaint service.
func NewService(s storage.Storage) *Service {
	return &Service{Storage: s}
}

// HandleComplaint processes a new complaint.
func (s *Service) HandleComplaint(complaint *models.Complaint) error {
	// Check if this is the first complaint against the user in this chat room
	existingComplaints, err := s.Storage.GetComplaintsByRoomAndReportedUser(complaint.RoomID, complaint.ReportedUserID)
	if err != nil {
		return err
	}
	if len(existingComplaints) > 1 {
		// Not the first complaint, so don't apply a penalty
		return nil
	}

	// Check for recent complaints from the same reporter to apply diminishing penalty
	recentComplaints, err := s.Storage.GetComplaintsByReporterSince(complaint.ReporterID, time.Now().Add(-1*time.Hour))
	if err != nil {
		return err
	}

	weight := float64(analysis.GetWeight(complaint.ComplaintType))
	for i := 0; i < len(recentComplaints); i++ {
		weight *= 0.75
	}
	finalWeight := int(weight)
	if finalWeight < 1 {
		finalWeight = 1
	}

	if err := s.Storage.UpdateUserReputation(complaint.ReportedUserID, -finalWeight); err != nil {
		return err
	}

	return s.CheckForBan(complaint.ReportedUserID)
}

// CheckForBan checks if a user should be banned based on their reputation and complaint history.
func (s *Service) CheckForBan(userID string) error {
	user, err := s.Storage.GetUserByID(userID)
	if err != nil {
		return err
	}

	// Threshold Ban
	if user.ReputationScore < config.BanThresholdReputation {
		return s.applyBan(user)
	}

	// Frequency Ban
	complaints, err := s.Storage.GetComplaintsForUser(userID, time.Now().Add(-config.BanFrequencyWindow))
	if err != nil {
		return err
	}
	if len(complaints) > config.BanThresholdFrequency {
		return s.applyBan(user)
	}

	return nil
}

func (s *Service) applyBan(user *models.User) error {
	lastBanDate, err := s.Storage.GetLastBanDate(user.ID)
	if err != nil {
		return err
	}

	level := 1
	if lastBanDate > 0 {
		if time.Since(time.Unix(lastBanDate, 0)) < 7*24*time.Hour {
			level = 2
		} else if time.Since(time.Unix(lastBanDate, 0)) < 30*24*time.Hour {
			level = 3
		}
	}

	duration := getBanDuration(level)
	user.IsBlocked = true
	user.BlockEndTime = time.Now().Add(duration).Unix()
	user.BlockLevel = level
	user.LastBanDate = time.Now().Unix()
	return s.Storage.UpdateUser(user)
}

func getBanDuration(level int) time.Duration {
	switch level {
	case 1:
		return config.BanLevel1Duration
	case 2:
		return config.BanLevel2Duration
	default:
		return config.BanLevel3Duration
	}
}
