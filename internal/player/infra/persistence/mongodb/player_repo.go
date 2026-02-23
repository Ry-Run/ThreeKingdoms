package mongodb

import (
	"context"
	"errors"

	"ThreeKingdoms/internal/player/entity"
	"ThreeKingdoms/internal/player/errs"
	"ThreeKingdoms/internal/player/infra/persistence/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const defaultPlayerCollectionName = "player"

const (
	OpLoadPlayer = "repo.player.LoadPlayer"
	OpSnapshot   = "repo.player.Save"
)

type PlayerRepo struct {
	coll *mongo.Collection
}

func NewPlayerRepo(db *mongo.Database) *PlayerRepo {
	if db == nil {
		return &PlayerRepo{}
	}
	return &PlayerRepo{coll: db.Collection(defaultPlayerCollectionName)}
}

func (r *PlayerRepo) LoadPlayer(ctx context.Context, id entity.PlayerID) (*entity.PlayerEntity, error) {
	if r == nil || r.coll == nil {
		return nil, errs.Wrap(OpLoadPlayer, errs.KindInfra, errors.New("mongodb player collection is nil"), nil)
	}

	var doc model.PlayerDoc
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err == nil {
		s := model.PlayerDocToState(doc)
		if s.PlayerID == 0 {
			s.PlayerID = id
		}
		return entity.HydratePlayerEntity(s), nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		initial := entity.PlayerEntity{}
		initial.SetPlayerID(id)
		return &initial, nil
	}
	return nil, err
}

func (r *PlayerRepo) Save(ctx context.Context, s *entity.PlayerEntitySnap) error {
	if s == nil {
		return nil
	}
	if r == nil || r.coll == nil {
		return errs.Wrap(OpSnapshot, errs.KindInfra, errors.New("mongodb player collection is nil"), nil)
	}

	doc := model.PlayerStateToDoc(s.State)
	if doc.PlayerID == 0 {
		return errs.Wrap(OpSnapshot, errs.KindInfra, entity.ErrPlayerNotFound, nil)
	}

	_, err := r.coll.ReplaceOne(
		ctx,
		bson.M{"_id": doc.PlayerID},
		doc,
		options.Replace().SetUpsert(true),
	)

	if err != nil {
		return errs.Wrap(OpSnapshot, errs.KindInfra, err, map[string]any{"player_id": doc.PlayerID})
	}
	return nil
}
