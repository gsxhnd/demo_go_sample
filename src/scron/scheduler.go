package scron

import (
	"context"
	"log"
	"sync"
	"time"
)

// Scheduler 是定时任务调度器
type Scheduler struct {
	mu    sync.RWMutex
	tasks map[string]*Task

	// ticker 用于每分钟检查任务的定时器
	ticker *time.Ticker

	// stopCh 用于通知调度器停止
	stopCh chan struct{}

	// doneCh 当调度器完全停止时会关闭
	doneCh chan struct{}

	// running 指示调度器是否正在运行
	running bool

	// checkInterval 设置检查任务的时间间隔（默认每分钟）
	checkInterval time.Duration

	// Logger 用于记录日志
	Logger *log.Logger

	// 防止重复启动
	startOnce sync.Once

	// 防止重复停止
	stopOnce sync.Once
}

// NewScheduler 创建一个新的调度器实例
func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks:         make(map[string]*Task),
		checkInterval: time.Minute,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		Logger:        log.Default(),
	}
}

// NewSchedulerWithInterval 创建一个使用自定义检查间隔的调度器
// interval: 检查任务的时间间隔，建议不小于 1 秒
func NewSchedulerWithInterval(interval time.Duration) *Scheduler {
	s := NewScheduler()
	s.checkInterval = interval
	return s
}

// AddTask 添加一个定时任务到调度器
// 如果任务ID已存在，则会覆盖原有任务
func (s *Scheduler) AddTask(task *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.ID] = task
	s.Logger.Printf("[Scheduler] 添加任务: %s (%s)\n", task.String(), task.TimeString())
}

// AddTaskQuick 快速添加任务
// hour: 执行小时（0-23）
// minute: 执行分钟（0-59）
// fn: 任务函数
func (s *Scheduler) AddTaskQuick(hour, minute int, fn TaskFunc) *Task {
	task := NewTask(hour, minute, fn)
	s.AddTask(task)
	return task
}

// AddTaskWithOptions 添加任务并设置选项
func (s *Scheduler) AddTaskWithOptions(hour, minute int, fn TaskFunc, opts ...TaskOption) *Task {
	task := NewTask(hour, minute, fn, opts...)
	s.AddTask(task)
	return task
}

// RemoveTask 根据ID移除任务
func (s *Scheduler) RemoveTask(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; exists {
		delete(s.tasks, id)
		s.Logger.Printf("[Scheduler] 移除任务: %s\n", id)
		return true
	}
	return false
}

// GetTask 根据ID获取任务
func (s *Scheduler) GetTask(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	return task, exists
}

// ListTasks 返回所有任务的快照
func (s *Scheduler) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// TaskCount 返回当前任务数量
func (s *Scheduler) TaskCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tasks)
}

// Start 启动调度器，开始执行定时任务
// 调度器会在后台运行，直到调用 Stop 或 context 被取消
func (s *Scheduler) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		s.mu.Lock()
		if s.running {
			s.mu.Unlock()
			return
		}
		s.running = true
		s.ticker = time.NewTicker(s.checkInterval)
		s.mu.Unlock()

		s.Logger.Printf("[Scheduler] 调度器启动，检查间隔: %v\n", s.checkInterval)

		go s.run(ctx)
	})
}

// run 是调度器的主循环
func (s *Scheduler) run(ctx context.Context) {
	defer close(s.doneCh)

	// 初始化所有任务的下次执行时间
	now := time.Now()
	s.initTasksNextRun(now)

	for {
		select {
		case <-ctx.Done():
			s.Logger.Println("[Scheduler] 收到上下文取消信号")
			return

		case <-s.stopCh:
			s.Logger.Println("[Scheduler] 收到停止信号")
			return

		case tickTime := <-s.ticker.C:
			s.checkAndRunTasks(tickTime)
		}
	}
}

// initTasksNextRun 初始化所有任务的下次执行时间
func (s *Scheduler) initTasksNextRun(now time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, task := range s.tasks {
		nextRun := task.NextRunTime(now)
		task.MarkScheduled(nextRun)
		s.Logger.Printf("[Scheduler] 任务 %s 下次执行时间: %s\n",
			task.String(), nextRun.Format("2006-01-02 15:04:05"))
	}
}

// checkAndRunTasks 检查并执行到期的任务
func (s *Scheduler) checkAndRunTasks(now time.Time) {
	s.mu.RLock()
	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	s.mu.RUnlock()

	for _, task := range tasks {
		if task.ShouldRun(now) {
			s.executeTask(task, now)
		}
	}
}

// executeTask 执行单个任务
func (s *Scheduler) executeTask(task *Task, now time.Time) {
	// 检查是否正在运行（防止重复执行）
	task.mu.RLock()
	lastRun := task.lastRun
	task.mu.RUnlock()

	// 如果上次执行距离现在不足 1 分钟，跳过（防止重复）
	if !lastRun.IsZero() && now.Sub(lastRun) < time.Minute {
		return
	}

	// 标记任务正在运行
	task.MarkRunning()

	s.Logger.Printf("[Scheduler] 执行任务: %s (%s)\n", task.String(), task.TimeString())

	// 使用 goroutine 执行任务，避免阻塞调度器
	go func(t *Task) {
		start := time.Now()
		defer func() {
			if r := recover(); r != nil {
				s.Logger.Printf("[Scheduler] 任务 %s 执行 panic: %v\n", t.String(), r)
			}
			s.Logger.Printf("[Scheduler] 任务 %s 执行完成，耗时: %v\n",
				t.String(), time.Since(start))
		}()

		// 执行任务函数
		t.Func()

		// 更新下次执行时间
		nextRun := t.NextRunTime(time.Now())
		t.MarkScheduled(nextRun)
		s.Logger.Printf("[Scheduler] 任务 %s 下次执行时间: %s\n",
			t.String(), nextRun.Format("2006-01-02 15:04:05"))
	}(task)
}

// Stop 停止调度器
// timeout: 等待任务完成的最大时间，0 表示不等待
func (s *Scheduler) Stop(timeout time.Duration) {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		if !s.running {
			s.mu.Unlock()
			return
		}
		s.running = false
		if s.ticker != nil {
			s.ticker.Stop()
		}
		s.mu.Unlock()

		close(s.stopCh)

		// 等待调度器完全停止
		if timeout > 0 {
			select {
			case <-s.doneCh:
				s.Logger.Println("[Scheduler] 调度器已停止")
			case <-time.After(timeout):
				s.Logger.Println("[Scheduler] 调度器停止超时")
			}
		} else {
			<-s.doneCh
			s.Logger.Println("[Scheduler] 调度器已停止")
		}
	})
}

// IsRunning 返回调度器是否正在运行
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Status 返回调度器的状态信息
func (s *Scheduler) Status() SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := SchedulerStatus{
		Running:   s.running,
		TaskCount: len(s.tasks),
		TaskList:  make([]TaskStatus, 0, len(s.tasks)),
	}

	for _, task := range s.tasks {
		status.TaskList = append(status.TaskList, TaskStatus{
			ID:       task.ID,
			Name:     task.String(),
			Time:     task.TimeString(),
			LastRun:  task.GetLastRun(),
			NextRun:  task.GetNextRun(),
			Timezone: task.getTimezone().String(),
		})
	}

	return status
}

// SchedulerStatus 调度器状态信息
type SchedulerStatus struct {
	Running   bool
	TaskCount int
	TaskList  []TaskStatus
}

// TaskStatus 单个任务的状态信息
type TaskStatus struct {
	ID       string
	Name     string
	Time     string
	LastRun  time.Time
	NextRun  time.Time
	Timezone string
}
