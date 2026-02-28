package dc

import (
	"context"
	"errors"
	"sync"
	"time"

	"ThreeKingdoms/internal/alliance/entity"
	"ThreeKingdoms/internal/alliance/service/port"
)

type AllianceID = entity.AllianceID

var (
	ErrClosed     = errors.New("alliance dc closed")
	ErrNoRepo     = errors.New("alliance repository is nil")
	ErrWriterDone = errors.New("alliance dc writer stopped")
)

type AllianceDC struct {
	repo       port.AllianceRepository
	entity     *entity.AllianceEntity
	flushEvery time.Duration

	mu sync.Mutex

	// coalescing：只保留最新快照
	pending *entity.AllianceEntitySnap

	// 版本控制
	version   uint64 // 已生成的最新版本
	persisted uint64 // 已成功落库的最新版本

	closed bool

	// 通知通道
	wake chan struct{} // 有 pending 可消费
	// persisted 推进广播：每次推进时 close(old)+new，一个推进可唤醒全部等待者
	persistNotify chan struct{}
	stop          chan struct{} // 请求停止 writer
	done          chan struct{} // writer 已退出
}

func NewAllianceDC(repo port.AllianceRepository) *AllianceDC {
	d := &AllianceDC{
		repo:          repo,
		flushEvery:    3 * time.Second,
		wake:          make(chan struct{}, 1),
		persistNotify: make(chan struct{}),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	go d.writerLoop()
	return d
}

func (d *AllianceDC) Load(ctx context.Context, allianceID AllianceID) (*entity.AllianceEntity, error) {
	if d.repo == nil {
		return nil, ErrNoRepo
	}
	alliance, err := d.repo.LoadAlliance(ctx, allianceID)
	if err != nil {
		return nil, err
	}
	d.entity = alliance
	return alliance, nil
}

func (d *AllianceDC) Entity() *entity.AllianceEntity { return d.entity }
func (d *AllianceDC) FlushEvery() time.Duration      { return d.flushEvery }

func (d *AllianceDC) IsDirty() bool {
	if d.entity == nil {
		return false
	}
	return d.entity.Dirty()
}

// Tick：生成快照并异步落库（不等待落库完成）
// 返回：本次生成的版本（0 表示无脏/无快照）
func (d *AllianceDC) Tick() (uint64, error) {
	if !d.IsDirty() {
		return 0, nil
	}
	if d.repo == nil {
		return 0, ErrNoRepo
	}
	s, ok, err := d.buildNextSnapshot()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	d.enqueueLatest(s)
	return s.Version, nil
}

// FlushSync：生成快照并阻塞等待“该版本（或更高版本）已成功落库”
func (d *AllianceDC) FlushSync(ctx context.Context) error {
	ctx = normalizeContext(ctx)

	target, err := d.Tick()
	if err != nil {
		return err
	}
	if target == 0 {
		return nil
	}
	return d.waitPersisted(ctx, target)
}

// WaitPersisted 等待指定版本（或更高版本）写库完成。
func (d *AllianceDC) WaitPersisted(ctx context.Context, target uint64) error {
	if target == 0 {
		return nil
	}
	return d.waitPersisted(normalizeContext(ctx), target)
}

func (d *AllianceDC) Close(ctx context.Context) error {
	ctx = normalizeContext(ctx)

	// 尽最大努力同步 flush（不保证一定成功，取决于 ctx）
	_ = d.FlushSync(ctx)

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

// ---------------- Internals: snapshot & enqueue ----------------

func (d *AllianceDC) buildNextSnapshot() (*entity.AllianceEntitySnap, bool, error) {
	if d.entity == nil || !d.IsDirty() {
		return nil, false, nil
	}

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, false, ErrClosed
	}
	d.version++
	v := d.version
	d.mu.Unlock()

	s := entity.NewAllianceEntitySnap(v, d.entity)
	d.entity.ClearDirty()
	return s, true, nil
}

func (d *AllianceDC) enqueueLatest(s *entity.AllianceEntitySnap) {
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

func (d *AllianceDC) popPending() *entity.AllianceEntitySnap {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.pending
	d.pending = nil
	return s
}

func (d *AllianceDC) requeueOnError(s *entity.AllianceEntitySnap) {
	if s == nil {
		return
	}

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	if s.Version < d.version {
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

func (d *AllianceDC) hasNewerVersion(version uint64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.version > version
}

// ---------------- Internals: persistence waiting ----------------

func (d *AllianceDC) waitPersisted(ctx context.Context, target uint64) error {
	for {
		d.mu.Lock()
		if d.persisted >= target {
			d.mu.Unlock()
			return nil
		}
		notify := d.persistNotify
		d.mu.Unlock()

		select {
		case <-notify:
			continue
		case <-d.done:
			d.mu.Lock()
			p2 := d.persisted
			d.mu.Unlock()
			if p2 >= target {
				return nil
			}
			return ErrWriterDone
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (d *AllianceDC) markPersisted(version uint64) {
	d.mu.Lock()
	if version <= d.persisted {
		d.mu.Unlock()
		return
	}
	d.persisted = version
	oldNotify := d.persistNotify
	d.persistNotify = make(chan struct{})
	d.mu.Unlock()

	close(oldNotify)
}

// ---------------- Writer loop ----------------

func (d *AllianceDC) writerLoop() {
	defer close(d.done)

	for {
		select {
		case <-d.wake:
			d.consumePending()
		case <-d.stop:
			// 尽可能消费最后一轮 pending（Close 已经先尝试 FlushSync）
			d.consumePending()
			return
		}
	}
}

func (d *AllianceDC) consumePending() {
	for {
		s := d.popPending()
		if s == nil {
			return
		}

		if d.hasNewerVersion(s.Version) {
			continue
		}

		if err := d.repo.Save(context.TODO(), s); err != nil {
			d.requeueOnError(s)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		d.markPersisted(s.Version)
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
