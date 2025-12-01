// Package analysis provides functionalities for analyzing user behavior and complaints.
// It includes logic for determining the severity of complaints and calculating their impact on user reputation.
package analysis

// ComplaintWeights defines the penalty values for different types of complaints.
var ComplaintWeights = map[string]int{
	"Low":      5,
	"Medium":   50,
	"Critical": 250,
}

// GetWeight returns the weight (penalty) for a given complaint type.
// It returns 0 if the complaint type is not recognized.
func GetWeight(complaintType string) int {
	return ComplaintWeights[complaintType]
}
