package mongo

import (
	"ThreeKingdoms/internal/shared/serverconfig"
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

func Open(cfg serverconfig.MongoDBConfig, l *zap.Logger) (*mongo.Client, error) {
	if cfg.URI == "" {
		return nil, errors.New("mongodb uri is empty")
	}
	if l == nil {
		l = zap.NewNop()
	}

	timeout := time.Duration(cfg.ConnectTimeoutS) * time.Second
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.URI))
	if err != nil {
		return nil, err
	}
	if err = client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}

	l.Info("open mongodb success",
		zap.String("uri", cfg.URI),
		zap.String("database", cfg.Database),
	)
	return client, nil
}
