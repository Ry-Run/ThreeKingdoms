package mongodb

import (
	"ThreeKingdoms/internal/alliance/entity"
	"ThreeKingdoms/internal/alliance/infra/persistence/model"
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const defaultCollectionName = "alliance"

type AllianceRepository struct {
	coll *mongo.Collection
}

func NewAllianceRepository(db *mongo.Database) *AllianceRepository {
	return &AllianceRepository{
		coll: db.Collection(defaultCollectionName),
	}
}

func (r *AllianceRepository) LoadAlliance(ctx context.Context, allianceID entity.AllianceID) (*entity.AllianceEntity, error) {
	if r == nil || r.coll == nil {
		return nil, errors.New("mongodb alliance collection is nil")
	}

	var doc model.AllianceDoc
	err := r.coll.FindOne(ctx, bson.M{"_id": allianceID}).Decode(&doc)
	if err == nil {
		s := model.AllianceDocToState(doc)
		return entity.HydrateAllianceEntity(s), nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		initial := entity.AllianceEntity{}
		initial.SetId(allianceID)
		initial.ReplaceMajors(make(map[entity.PlayerID]entity.MajorState))
		initial.ReplaceMembers(make(map[entity.PlayerID]entity.MemberState))
		return &initial, nil
	}
	return nil, err
}

func (r *AllianceRepository) ListAllianceSummaryByWorld(ctx context.Context, worldID entity.WorldID) ([]entity.AllianceState, error) {
	if r == nil || r.coll == nil {
		return nil, errors.New("mongodb alliance collection is nil")
	}
	cur, err := r.coll.Find(ctx, bson.M{"world_id": worldID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	out := make([]entity.AllianceState, 0)
	for cur.Next(ctx) {
		var doc model.AllianceDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, model.AllianceDocToState(doc))
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AllianceRepository) Save(ctx context.Context, s *entity.AllianceEntitySnap) error {
	if s == nil {
		return nil
	}
	if r == nil || r.coll == nil {
		return errors.New("mongodb alliance collection is nil")
	}

	state := s.State
	doc := model.AllianceStateToDoc(state)
	_, err := r.coll.ReplaceOne(
		ctx,
		bson.M{"_id": doc.Id},
		doc,
		options.Replace().SetUpsert(true),
	)
	return err
}
