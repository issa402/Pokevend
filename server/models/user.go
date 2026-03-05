// models/user.go — User domain type
package models

import "time"

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName *string   `json:"displayName"`
	ZipCode     *string   `json:"zipCode"`
	CreatedAt   time.Time `json:"createdAt"`
}
