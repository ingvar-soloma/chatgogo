// Package analysis provides functionalities for analyzing user behavior and complaints.
// It includes logic for determining the severity of complaints and calculating their impact on user reputation.
package analysis

import "chatgogo/backend/internal/config"

// GetWeight returns the weight (penalty) for a given complaint type.
// It returns 0 if the complaint type is not recognized.
func GetWeight(complaintType string) int {
	return config.ComplaintWeights[complaintType]
}
