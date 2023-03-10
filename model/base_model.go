package model

import (
	"time"
)

// BaseModel is similar to the gorm.Model and it includes the
// ID as a string, CreatedAt and UpdatedAt fields
type BaseModel struct {
	ID        string    `gorm:"primaryKey" json:"id" bson:"_id"`
	CreatedAt time.Time `gorm:"index" json:"createdAt" bson:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updated_at"`
}

// Success is the default response for successful operation
// Returns true without the JSON
type Success struct {
	// Only returns true, not a json object
	Success bool `json:"success"`
} //@name SuccessResponse
