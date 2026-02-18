package dc

import (
	"ThreeKingdoms/internal/player/app/port"
	"ThreeKingdoms/internal/player/entity"
	"context"
	"sync"
	"time"
)

type PlayerID = entity.PlayerID

type PlayerDC struct {
	repo       port.PlayerRepository
	entity     *entity.Player
	flushEvery time.Duration

	mu      sync.Mutex
	pending *entity.PlayerPersistSnapshot
	version uint64
	closed  bool

	wake chan struct{}
	stop chan struct{}
	done chan struct{}
}

func NewPlayerDC(repo port.PlayerRepository) *PlayerDC {
	d := &PlayerDC{
		repo:       repo,
		flushEvery: 3000 * time.Millisecond,
		wake:       make(chan struct{}, 1),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	go d.writerLoop()
	return d
}

func (d *PlayerDC) Load(ctx context.Context, playerID *PlayerID) (*entity.Player, error) {
	player, err := d.repo.LoadPlayer(ctx, playerID)
	if err != nil {
		return nil, err
	}
	d.entity = player
	return player, nil
}

// load 是全量加载数据到内存；flush 采用脏检查 + 同步快照 + 异步写库
// 当前持久化粒度是“脏表整行保存”（role/resource 各自一行），不是列级更新
// 仍需注意缓存一致性：如果系统里存在“绕过 dc 的写”（比如 GM），可能产生覆盖问题
// 解决建议：统一经 actor 命令改状态，或引入版本号/CAS 防止旧值覆盖新值
func (d *PlayerDC) Flush(ctx context.Context) {
	if !d.IsDirty() {
		return
	}
	s, ok := d.buildNextSnapshot()
	if !ok {
		return
	}
	d.enqueueLatest(s)
	return
}

func (d *PlayerDC) IsDirty() bool {
	if d.entity == nil {
		return false
	}
	return d.entity.Dirty()
}

func (d *PlayerDC) ClearDirty() {
	if d.entity == nil {
		return
	}
	d.entity.ClearDirty()
}

func (d *PlayerDC) Entity() *entity.Player {
	return d.entity
}

func (d *PlayerDC) FlushEvery() time.Duration {
	return d.flushEvery
}

func (d *PlayerDC) Close(ctx context.Context) error {
	d.Flush(ctx)

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

func (d *PlayerDC) buildNextSnapshot() (*entity.PlayerPersistSnapshot, bool) {
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

func (d *PlayerDC) enqueueLatest(s *entity.PlayerPersistSnapshot) {
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

func (d *PlayerDC) popPending() *entity.PlayerPersistSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.pending
	d.pending = nil
	return s
}

func (d *PlayerDC) requeueOnError(s *entity.PlayerPersistSnapshot) {
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

func (d *PlayerDC) writerLoop() {
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

func (d *PlayerDC) consumePending() {
	for {
		s := d.popPending()
		if s == nil {
			return
		}
		if err := d.repo.Snapshot(context.TODO(), s); err != nil {
			// 写库失败时重排当前快照；若已有更新快照，会被更高 version 覆盖。
			d.requeueOnError(s)
			time.Sleep(200 * time.Millisecond)
			continue
		}
	}
}
