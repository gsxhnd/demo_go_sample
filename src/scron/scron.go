package scron

import (
	"sync"
	"time"
)

// TaskFunc 是任务执行函数的类型定义
type TaskFunc func()

// Task 代表一个定时任务
type Task struct {
	// ID 是任务的唯一标识符
	ID string

	// Name 是任务的名称（可选，用于日志等）
	Name string

	// Hour 指定任务执行的小时（0-23）
	Hour int

	// Minute 指定任务执行的分钟（0-59）
	Minute int

	// Timezone 指定任务执行的时区，nil 表示使用本地时区
	Timezone *time.Location

	// Func 是任务执行函数
	Func TaskFunc

	//mu 保护 lastRun 和 nextRun
	mu sync.RWMutex

	// lastRun 记录上次执行时间
	lastRun time.Time

	// nextRun 记录下次执行时间
	nextRun time.Time
}

// NewTask 创建一个新的每日定时任务
// hour: 执行小时（0-23）
// minute: 执行分钟（0-59）
// fn: 任务执行函数
// opts: 可选配置（ID、Name、Timezone）
func NewTask(hour, minute int, fn TaskFunc, opts ...TaskOption) *Task {
	task := &Task{
		Hour:   hour,
		Minute: minute,
		Func:   fn,
	}

	// 应用可选配置
	for _, opt := range opts {
		opt(task)
	}

	// 设置默认值
	if task.ID == "" {
		task.ID = generateID()
	}

	return task
}

// TaskOption 是任务配置选项函数类型
type TaskOption func(*Task)

// WithID 设置任务ID
func WithID(id string) TaskOption {
	return func(t *Task) {
		t.ID = id
	}
}

// WithName 设置任务名称
func WithName(name string) TaskOption {
	return func(t *Task) {
		t.Name = name
	}
}

// WithTimezone 设置任务执行的时区
func WithTimezone(loc *time.Location) TaskOption {
	return func(t *Task) {
		t.Timezone = loc
	}
}

// getTimezone 返回任务使用的时区，如果未设置则返回本地时区
func (t *Task) getTimezone() *time.Location {
	if t.Timezone != nil {
		return t.Timezone
	}
	return time.Local
}

// NextRunTime 计算任务的下次执行时间
// 基于当前时间计算距离下一次在指定时间点执行的剩余时间
func (t *Task) NextRunTime(now time.Time) time.Time {
	loc := t.getTimezone()

	// 将当前时间转换到任务时区
	nowInLoc := now.In(loc)

	// 构建今天的目标时间
	todayTarget := time.Date(
		nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(),
		t.Hour, t.Minute, 0, 0, loc,
	)

	// 如果今天的时间已过，则计算明天的目标时间
	nextRun := todayTarget
	if !nowInLoc.Before(todayTarget) {
		nextRun = todayTarget.Add(24 * time.Hour)
	}

	return nextRun
}

// ShouldRun 判断任务是否应该在指定时间执行
// 考虑到秒级精度，只在同一分钟内触发一次
func (t *Task) ShouldRun(now time.Time) bool {
	loc := t.getTimezone()
	nowInLoc := now.In(loc)

	return nowInLoc.Hour() == t.Hour && nowInLoc.Minute() == t.Minute
}

// MarkRunning 标记任务正在运行
func (t *Task) MarkRunning() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastRun = time.Now()
}

// MarkScheduled 标记任务已调度
func (t *Task) MarkScheduled(nextRun time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.nextRun = nextRun
}

// GetLastRun 获取任务上次执行时间
func (t *Task) GetLastRun() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastRun
}

// GetNextRun 获取任务下次执行时间
func (t *Task) GetNextRun() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.nextRun
}

// String 返回任务的字符串表示
func (t *Task) String() string {
	name := t.Name
	if name == "" {
		name = t.ID
	}
	return name
}

// TimeString 返回任务执行时间的字符串表示
func (t *Task) TimeString() string {
	return formatTime(t.Hour, t.Minute)
}

// generateID 生成唯一的任务ID
func generateID() string {
	return time.Now().Format("20060102150405.000000")
}

// formatTime 格式化时间字符串
func formatTime(hour, minute int) string {
	return time.Date(0, 0, 0, hour, minute, 0, 0, time.UTC).Format("15:04")
}
