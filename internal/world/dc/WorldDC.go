package dc

import (
	"ThreeKingdoms/internal/world/app/port"
	"ThreeKingdoms/internal/world/entity"
	"context"
	"errors"
	"sync"
	"time"
)

type WorldID = entity.WorldID

type WorldDC struct {
	repo       port.WorldRepository
	entity     *entity.World
	flushEvery time.Duration

	mu      sync.Mutex
	pending *entity.WorldPersistSnapshot
	version uint64
	closed  bool

	wake chan struct{}
	stop chan struct{}
	done chan struct{}
}

func NewWorldDC(repo port.WorldRepository) *WorldDC {
	d := &WorldDC{
		repo:       repo,
		flushEvery: 3000 * time.Millisecond,
		wake:       make(chan struct{}, 1),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	go d.writerLoop()
	return d
}

func (d *WorldDC) Load(ctx context.Context, worldID *WorldID) (*entity.World, error) {
	if d.repo == nil {
		return nil, errors.New("world repository is nil")
	}
	world, err := d.repo.LoadWorld(ctx, worldID)
	if err != nil {
		return nil, err
	}
	d.entity = world
	return world, nil
}

func (d *WorldDC) Flush(ctx context.Context) error {
	if !d.IsDirty() {
		return nil
	}
	if d.repo == nil {
		return errors.New("world repository is nil")
	}
	s, ok := d.buildNextSnapshot()
	if !ok {
		return nil
	}
	d.enqueueLatest(s)
	return nil
}

func (d *WorldDC) IsDirty() bool {
	if d.entity == nil {
		return false
	}
	return d.entity.Dirty()
}

func (d *WorldDC) ClearDirty() {
	if d.entity == nil {
		return
	}
	d.entity.ClearDirty()
}

func (d *WorldDC) Entity() *entity.World {
	return d.entity
}

func (d *WorldDC) FlushEvery() time.Duration {
	return d.flushEvery
}

func (d *WorldDC) Close(ctx context.Context) error {
	_ = d.Flush(ctx)

	d.mu.Lock()
	if !d.closed {
		d.closed = true
		close(d.stop)
	}
	d.mu.Unlock()

	select {
	case <-d.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *WorldDC) buildNextSnapshot() (*entity.WorldPersistSnapshot, bool) {
	if d.entity == nil {
		return nil, false
	}
	d.mu.Lock()
	d.version++
	version := d.version
	d.mu.Unlock()

	s, ok := d.entity.BuildPersistSnapshot(version)
	if !ok {
		return nil, false
	}
	d.entity.ClearDirty()
	return s, true
}

func (d *WorldDC) enqueueLatest(s *entity.WorldPersistSnapshot) {
	if s == nil {
		return
	}

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	if d.pending == nil || d.pending.Version < s.Version {
		d.pending = s
	}
	d.mu.Unlock()

	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *WorldDC) popPending() *entity.WorldPersistSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.pending
	d.pending = nil
	return s
}

func (d *WorldDC) requeueOnError(s *entity.WorldPersistSnapshot) {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	if d.pending == nil || d.pending.Version < s.Version {
		d.pending = s
	}
	d.mu.Unlock()

	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *WorldDC) writerLoop() {
	defer close(d.done)

	for {
		select {
		case <-d.wake:
			d.consumePending()
		case <-d.stop:
			d.consumePending()
			return
		}
	}
}

func (d *WorldDC) consumePending() {
	for {
		s := d.popPending()
		if s == nil {
			return
		}
		if err := d.repo.Save(context.TODO(), s); err != nil {
			// 写库失败时重排当前快照；若已有更新快照，会被更高 version 覆盖。
			d.requeueOnError(s)
			time.Sleep(200 * time.Millisecond)
			continue
		}
	}
}
