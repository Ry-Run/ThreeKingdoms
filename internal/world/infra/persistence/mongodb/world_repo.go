package mongodb

import (
	"ThreeKingdoms/internal/world/entity"
	"ThreeKingdoms/internal/world/infra/persistence/model"
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const defaultCollectionName = "world"

type PlayerID = entity.PlayerID
type CityID = entity.CityID

type WorldRepository struct {
	coll *mongo.Collection
}

func NewWorldRepository(db *mongo.Database) *WorldRepository {
	return &WorldRepository{
		coll: db.Collection(defaultCollectionName),
	}
}

func (r *WorldRepository) LoadWorld(ctx context.Context, id entity.WorldID) (*entity.WorldEntity, error) {
	if r == nil || r.coll == nil {
		return nil, errors.New("mongodb world collection is nil")
	}

	var doc model.WorldDoc
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err == nil {
		s := model.WorldDocToState(doc)
		return entity.HydrateWorldEntity(s), nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		initial := entity.WorldEntity{}
		initial.SetWorldId(id)
		initial.ReplaceCityByPlayer(make(map[PlayerID]map[CityID]entity.CityState))
		return &initial, nil
	}
	return nil, err
}

func (r *WorldRepository) Save(ctx context.Context, s *entity.WorldEntitySnap) error {
	if s == nil {
		return nil
	}
	if r == nil || r.coll == nil {
		return errors.New("mongodb world collection is nil")
	}

	world := s.State
	doc := model.WorldStateToDoc(world)

	_, err := r.coll.ReplaceOne(
		ctx,
		bson.M{"_id": doc.WorldId},
		doc,
		options.Replace().SetUpsert(true),
	)
	return err
}
