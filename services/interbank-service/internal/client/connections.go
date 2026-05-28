package client

import (
	"context"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
)

// UserServiceConn is a typed wrapper so fx can distinguish the user-service
// connection from the trading-service one in the dependency graph.
type UserServiceConn struct{ *grpc.ClientConn }

// TradingServiceConn is the trading-service counterpart of UserServiceConn.
type TradingServiceConn struct{ *grpc.ClientConn }

func NewUserServiceConnection(lc fx.Lifecycle, cfg *config.Configuration) (*UserServiceConn, error) {
	conn, err := grpc.NewClient(
		cfg.UserServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return conn.Close()
		},
	})

	return &UserServiceConn{ClientConn: conn}, nil
}

func NewTradingServiceConnection(lc fx.Lifecycle, cfg *config.Configuration) (*TradingServiceConn, error) {
	conn, err := grpc.NewClient(
		cfg.TradingServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return conn.Close()
		},
	})

	return &TradingServiceConn{ClientConn: conn}, nil
}
