package mysql

import (
	"ThreeKingdoms/internal/player/entity"
	"ThreeKingdoms/internal/player/errs"
	"ThreeKingdoms/internal/player/infra/persistence/mapper"
	"ThreeKingdoms/internal/player/infra/persistence/model"
	"context"
	"errors"

	"gorm.io/gorm"
)

type PlayerRepo struct {
	db *gorm.DB
}

func NewPlayerRepo(db *gorm.DB) *PlayerRepo {
	return &PlayerRepo{db: db}
}

func (r *PlayerRepo) LoadPlayer(ctx context.Context, id *entity.PlayerID) (*entity.Player, error) {
	role, err := r.GetRoleByRID(ctx, int(*id))
	if err != nil {
		return nil, err
	}
	resource, err := r.GetResourceByRID(ctx, int(*id))
	if err != nil {
		return nil, err
	}
	return entity.NewPlayer(id, role, resource), nil
}

func (r *PlayerRepo) WithTx(tx *gorm.DB) *PlayerRepo {
	return &PlayerRepo{
		db: tx,
	}
}

const OpGetRoleByRID = "repo.player.GetRoleByRID"

func (r *PlayerRepo) GetRoleByRID(ctx context.Context, rid int) (*entity.RoleEntity, error) {
	var m model.Role
	err := r.db.WithContext(ctx).Where("rid = ?", rid).First(&m).Error

	switch {
	case err == nil:
		return mapper.RoleModelToEntity(&m), nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, entity.ErrRoleNotFound
	default:
		//  纯技术错误（连接超时等），是无法转换的技术错误，保持原样或包装返回给上级
		return nil, errs.Wrap(OpGetRoleByRID, errs.KindInfra, err, map[string]any{"rid": rid})
	}
}

const OpSaveRole = "repo.player.SaveRole"

const OpSaveResource = "repo.player.SaveResource"

func (r *PlayerRepo) Snapshot(ctx context.Context, s *entity.PlayerPersistSnapshot) error {
	if s == nil {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := r.WithTx(tx)
		if s.SaveRole {
			if err := txRepo.saveRole(ctx, s.Role); err != nil {
				return err
			}
		}
		if s.SaveResource {
			if err := txRepo.saveResource(ctx, s.Resource); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PlayerRepo) saveRole(ctx context.Context, s entity.RoleEntitySnapshot) error {
	m := &model.Role{
		Rid:        s.Rid,
		Uid:        s.Uid,
		Headid:     s.Headid,
		Sex:        s.Sex,
		NickName:   s.NickName,
		Balance:    s.Balance,
		LoginTime:  s.LoginTime,
		LogoutTime: s.LogoutTime,
		Profile:    s.Profile,
		CreatedAt:  s.CreatedAt,
	}

	err := r.db.WithContext(ctx).Save(m).Error
	if err != nil {
		return errs.Wrap(OpSaveRole, errs.KindInfra, err, map[string]any{"rid": s.Rid})
	}
	return nil
}

func (r *PlayerRepo) saveResource(ctx context.Context, s entity.ResourceEntitySnapshot) error {
	m := &model.Resource{
		Id:     s.Id,
		Rid:    s.Rid,
		Wood:   s.Wood,
		Iron:   s.Iron,
		Stone:  s.Stone,
		Grain:  s.Grain,
		Gold:   s.Gold,
		Decree: s.Decree,
	}

	err := r.db.WithContext(ctx).Save(m).Error
	if err != nil {
		return errs.Wrap(OpSaveResource, errs.KindInfra, err, map[string]any{"rid": s.Rid})
	}
	return nil
}

const OpGetResourceByRID = "repo.player.GetResourceByRID"

func (r *PlayerRepo) GetResourceByRID(ctx context.Context, rid int) (*entity.ResourceEntity, error) {
	var m model.Resource
	err := r.db.WithContext(ctx).Where("rid = ?", rid).First(&m).Error

	switch {
	case err == nil:
		return mapper.ResourceModelToEntity(&m), nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, entity.ErrRoleNotFound
	default:
		//  纯技术错误（连接超时等），是无法转换的技术错误，保持原样或包装返回给上级
		return nil, errs.Wrap(OpGetResourceByRID, errs.KindInfra, err, map[string]any{"rid": rid})
	}
}

const OpGetRoleByUid = "repo.player.GetRoleByUid"

func (r *PlayerRepo) GetRoleByUid(ctx context.Context, uid int) (*entity.RoleEntity, error) {
	var m model.Role
	err := r.db.WithContext(ctx).Where("uid = ?", uid).First(&m).Error

	switch {
	case err == nil:
		return mapper.RoleModelToEntity(&m), nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, entity.ErrRoleNotFound
	default:
		//  纯技术错误（连接超时等），是无法转换的技术错误，保持原样或包装返回给上级
		return nil, errs.Wrap(OpGetRoleByUid, errs.KindInfra, err, map[string]any{"uid": uid})
	}
}
