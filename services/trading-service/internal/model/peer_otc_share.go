package model

import "time"

type PeerOtcShareReservationStatus string

const (
	PeerOtcShareReservationActive   PeerOtcShareReservationStatus = "ACTIVE"
	PeerOtcShareReservationReleased PeerOtcShareReservationStatus = "RELEASED"
	PeerOtcShareReservationConsumed PeerOtcShareReservationStatus = "CONSUMED"
)

type PeerOtcShareReservation struct {
	ContractID     string                        `gorm:"primaryKey;size:128"`
	SellerID       uint                          `gorm:"not null;index"`
	OwnerType      OwnerType                     `gorm:"not null;size:10;default:'CLIENT'"`
	StockAssetID   uint                          `gorm:"not null;index"`
	ReservedAmount float64                       `gorm:"not null"`
	Status         PeerOtcShareReservationStatus `gorm:"not null;size:20"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PeerOtcShareCredit struct {
	ContractID      string    `gorm:"primaryKey;size:128"`
	BuyerID         uint      `gorm:"not null;index"`
	OwnerType       OwnerType `gorm:"not null;size:10;default:'CLIENT'"`
	StockAssetID    uint      `gorm:"not null;index"`
	Amount          float64
	PricePerUnitRSD float64
	CreatedAt       time.Time
}
