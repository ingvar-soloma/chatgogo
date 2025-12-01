package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"chatgogo/backend/internal/storage"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	storageSvc := storage.NewStorageService(db, nil) // No redis needed for admin CLI

	if len(os.Args) < 2 {
		fmt.Println("Usage: admin <command> [args]")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "ban":
		if len(os.Args) < 3 {
			fmt.Println("Usage: admin ban <user_id> [duration_in_hours]")
			os.Exit(1)
		}
		userID := os.Args[2]
		var duration int
		if len(os.Args) > 3 {
			var err error
			duration, err = strconv.Atoi(os.Args[3])
			if err != nil {
				fmt.Println("Invalid duration. Please provide an integer.")
				os.Exit(1)
			}
		}
		if err := banUser(storageSvc, userID, duration); err != nil {
			log.Fatalf("Error banning user: %v", err)
		}
		fmt.Printf("User %s has been banned.\n", userID)
	case "unban":
		if len(os.Args) != 3 {
			fmt.Println("Usage: admin unban <user_id>")
			os.Exit(1)
		}
		userID := os.Args[2]
		if err := unbanUser(storageSvc, userID); err != nil {
			log.Fatalf("Error unbanning user: %v", err)
		}
		fmt.Printf("User %s has been unbanned.\n", userID)
	case "confirm-complaint":
		if len(os.Args) != 3 {
			fmt.Println("Usage: admin confirm-complaint <complaint_id>")
			os.Exit(1)
		}
		complaintIDStr := os.Args[2]
		complaintID, err := strconv.Atoi(complaintIDStr)
		if err != nil {
			fmt.Println("Invalid complaint ID. Please provide an integer.")
			os.Exit(1)
		}
		if err := confirmComplaint(storageSvc, uint(complaintID)); err != nil {
			log.Fatalf("Error confirming complaint: %v", err)
		}
		fmt.Printf("Complaint %s has been confirmed.\n", complaintIDStr)
	default:
		fmt.Println("Unknown command")
		os.Exit(1)
	}
}

func banUser(s storage.Storage, userID string, duration int) error {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return err
	}
	user.IsBlocked = true
	if duration > 0 {
		user.BlockEndTime = time.Now().Add(time.Duration(duration) * time.Hour).Unix()
	}
	return s.UpdateUser(user)
}

func unbanUser(s storage.Storage, userID string) error {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return err
	}
	user.IsBlocked = false
	user.BlockEndTime = 0
	return s.UpdateUser(user)
}

func confirmComplaint(s storage.Storage, complaintID uint) error {
	complaint, err := s.GetComplaintByID(complaintID)
	if err != nil {
		return err
	}
	return s.UpdateUserReputation(complaint.ReporterID, 50)
}
