package model

import "time"

type FundHistoryRecord struct {
	FundHistoryRecordID uint      `gorm:"primaryKey;autoIncrement"`
	FundID              uint      `gorm:"not null;index"`
	Fund                InvestmentFund
	RecordDate          time.Time `gorm:"not null"`
	TotalValue          float64   `gorm:"not null"`
	Profit              float64   `gorm:"not null"`
	LiquidAssets        float64   `gorm:"not null"`
}
