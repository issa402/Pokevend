package models
import "time"

type PriceAlertSettings struct {
	ID string `json:"id"`
	UserID string `json:"userId"`
	CardName string `json:"cardName"`
	Threshold float64 `json:"threshold"`
	Direction string `json:"direction"`
	IsActive bool `json:"is_Active"`
	CreatedAt time.Time `json:"time"`

}