// models/show.go — Show domain type
package models

import "time"

type Show struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	VenueName  *string    `json:"venueName"`
	City       *string    `json:"city"`
	State      *string    `json:"state"`
	StartDate  *time.Time `json:"startDate"`
	EndDate    *time.Time `json:"endDate"`
	EventURL   *string    `json:"eventUrl"`
}
