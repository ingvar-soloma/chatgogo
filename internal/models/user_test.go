package models_test

import (
	"chatgogo/backend/internal/models"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

// TestUserBeforeCreate_GeneratesUUID verifies that the BeforeCreate hook generates a valid UUID.
func TestUserBeforeCreate_GeneratesUUID(t *testing.T) {
	// Arrange
	user := &models.User{
		TelegramID:  "123456789",
		Age:         25,
		Gender:      "female",
		Interests:   pq.StringArray{"music", "travel", "coding"},
		RatingScore: 0,
	}

	// Ensure ID is empty before hook
	assert.Empty(t, user.ID, "User ID should be empty before BeforeCreate")

	// Act - Call the hook directly (GORM would call this automatically)
	err := user.BeforeCreate(nil) // nil *gorm.DB is acceptable for this hook

	// Assert
	assert.NoError(t, err, "BeforeCreate should not return an error")
	assert.NotEmpty(t, user.ID, "User ID must be populated after BeforeCreate")

	// Verify it's a valid UUID
	parsedUUID, parseErr := uuid.Parse(user.ID)
	assert.NoError(t, parseErr, "User ID must be a valid UUID string")
	assert.NotEqual(t, uuid.Nil, parsedUUID, "Generated UUID should not be nil UUID")
}

// TestUserBeforeCreate_PreservesExistingID verifies that the hook doesn't overwrite an existing ID.
func TestUserBeforeCreate_PreservesExistingID(t *testing.T) {
	// Arrange
	existingID := uuid.New().String()
	user := &models.User{
		ID:          existingID,
		TelegramID:  "987654321",
		Age:         30,
		Gender:      "male",
		Interests:   pq.StringArray{"sports", "movies"},
		RatingScore: 5,
	}

	// Act
	err := user.BeforeCreate(nil)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, existingID, user.ID, "BeforeCreate should preserve existing ID")
}

// TestUserBeforeCreate_MultipleUsers verifies unique UUIDs are generated for multiple users.
func TestUserBeforeCreate_MultipleUsers(t *testing.T) {
	// Arrange
	users := []*models.User{
		{TelegramID: "111", Age: 20, Gender: "female"},
		{TelegramID: "222", Age: 22, Gender: "male"},
		{TelegramID: "333", Age: 24, Gender: "non-binary"},
	}

	generatedIDs := make(map[string]bool)

	// Act
	for _, user := range users {
		err := user.BeforeCreate(nil)
		assert.NoError(t, err)

		// Assert uniqueness
		assert.NotContains(t, generatedIDs, user.ID, "Each user should have a unique ID")
		generatedIDs[user.ID] = true

		// Verify valid UUID
		_, parseErr := uuid.Parse(user.ID)
		assert.NoError(t, parseErr)
	}

	// Assert all IDs are different
	assert.Equal(t, len(users), len(generatedIDs), "All generated IDs should be unique")
}

// TestUserStructTags verifies that struct tags are correctly defined for GORM and JSON.
func TestUserStructTags(t *testing.T) {
	// This test uses reflection to verify struct tags are present
	// (useful for catching accidental tag removal during refactoring)

	user := models.User{}
	userType := reflect.TypeOf(user)

	// Check ID field
	idField, found := userType.FieldByName("ID")
	assert.True(t, found, "ID field should exist")
	assert.Contains(t, idField.Tag.Get("gorm"), "primaryKey", "ID should be marked as primary key")
	assert.Contains(t, idField.Tag.Get("json"), "id", "ID should have json tag")

	// Check TelegramID field
	tgField, found := userType.FieldByName("TelegramID")
	assert.True(t, found, "TelegramID field should exist")
	assert.Contains(t, tgField.Tag.Get("gorm"), "uniqueIndex", "TelegramID should have unique index")

	// Check Interests field (should use PostgreSQL array type)
	interestsField, found := userType.FieldByName("Interests")
	assert.True(t, found, "Interests field should exist")
	assert.Contains(t, interestsField.Tag.Get("gorm"), "type:text[]", "Interests should use PostgreSQL array type")
}

// TestUserValidation_EdgeCases tests edge cases for user data.
func TestUserValidation_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		user        models.User
		expectValid bool
		description string
	}{
		{
			name: "Empty Telegram ID",
			user: models.User{
				TelegramID:  "",
				Age:         25,
				Gender:      "female",
				RatingScore: 0,
			},
			expectValid: true, // TelegramID can be empty (user might be WebSocket-only)
			description: "TelegramID can be empty for non-Telegram users",
		},
		{
			name: "Zero Age",
			user: models.User{
				TelegramID:  "123",
				Age:         0,
				Gender:      "prefer not to say",
				RatingScore: 0,
			},
			expectValid: true, // No validation in current model
			description: "Age 0 is accepted (no explicit validation in model)",
		},
		{
			name: "Negative Rating",
			user: models.User{
				TelegramID:  "456",
				Age:         30,
				Gender:      "male",
				RatingScore: -5,
			},
			expectValid: true, // No validation prevents negative ratings
			description: "Negative rating accepted (consider adding validation)",
		},
		{
			name: "Empty Interests Array",
			user: models.User{
				TelegramID:  "789",
				Age:         28,
				Gender:      "female",
				Interests:   pq.StringArray{},
				RatingScore: 3,
			},
			expectValid: true,
			description: "Empty interests array is valid",
		},
		{
			name: "Nil Interests Array",
			user: models.User{
				TelegramID:  "101112",
				Age:         35,
				Gender:      "male",
				Interests:   nil,
				RatingScore: 4,
			},
			expectValid: true,
			description: "Nil interests array is valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// BeforeCreate should work for all cases
			err := tt.user.BeforeCreate(nil)
			assert.NoError(t, err, tt.description)
			assert.NotEmpty(t, tt.user.ID, "ID should be generated")

			// Note: This test documents the lack of validation.
			// Production code should add validation for:
			// - Age > 0 && Age < 120
			// - RatingScore >= 0
			// - TelegramID format validation
		})
	}
}

// TestUserInterestsArray verifies PostgreSQL array functionality.
func TestUserInterestsArray(t *testing.T) {
	// Arrange
	interests := pq.StringArray{"reading", "hiking", "photography"}
	user := &models.User{
		TelegramID:  "array_test",
		Age:         27,
		Gender:      "non-binary",
		Interests:   interests,
		RatingScore: 5,
	}

	// Act
	err := user.BeforeCreate(nil)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 3, len(user.Interests), "Should have 3 interests")
	assert.Contains(t, user.Interests, "reading")
	assert.Contains(t, user.Interests, "hiking")
	assert.Contains(t, user.Interests, "photography")
}

// BenchmarkUserBeforeCreate measures UUID generation performance.
func BenchmarkUserBeforeCreate(b *testing.B) {
	user := &models.User{
		TelegramID:  "benchmark_user",
		Age:         25,
		Gender:      "female",
		RatingScore: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user.ID = "" // Reset ID
		_ = user.BeforeCreate(nil)
	}
}
