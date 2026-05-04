package job

import (
	"context"
	"log"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type FundHistoryJob struct {
	svc             *service.InvestmentFundService
	fundHistoryRepo repository.FundHistoryRecordRepository
}

func NewFundHistoryJob(svc *service.InvestmentFundService, repo repository.FundHistoryRecordRepository) *FundHistoryJob {
	return &FundHistoryJob{svc: svc, fundHistoryRepo: repo}
}

func (j *FundHistoryJob) Run(ctx context.Context) error {
	log.Println("Starting Fund History Job")
	if err := j.svc.CalculateAndSaveDailyHistory(ctx, j.fundHistoryRepo); err != nil {
		log.Printf("Failed to calculate and save daily fund history: %v", err)
		return err
	}
	log.Println("Fund History Job completed successfully")
	return nil
}
