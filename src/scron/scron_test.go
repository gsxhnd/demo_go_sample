package scron

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============== Task 测试 ==============

func TestNewTask(t *testing.T) {
	fn := func() {}
	task := NewTask(10, 30, fn)

	if task.Hour != 10 {
		t.Errorf("期望 Hour=10，实际=%d", task.Hour)
	}
	if task.Minute != 30 {
		t.Errorf("期望 Minute=30，实际=%d", task.Minute)
	}
	if task.Func == nil {
		t.Error("Func 不应为 nil")
	}
}

func TestNewTaskWithOptions(t *testing.T) {
	fn := func() {}
	task := NewTask(14, 0, fn,
		WithID("custom-id"),
		WithName("每日任务"),
		WithTimezone(time.UTC),
	)

	if task.ID != "custom-id" {
		t.Errorf("期望 ID=custom-id，实际=%s", task.ID)
	}
	if task.Name != "每日任务" {
		t.Errorf("期望 Name=每日任务，实际=%s", task.Name)
	}
	if task.Timezone != time.UTC {
		t.Error("Timezone 应为 UTC")
	}
}

func TestTaskShouldRun(t *testing.T) {
	tests := []struct {
		name      string
		hour      int
		minute    int
		checkTime time.Time
		expected  bool
	}{
		{
			name:      "应该运行",
			hour:      10,
			minute:    30,
			checkTime: time.Date(2024, 1, 1, 10, 30, 15, 0, time.Local),
			expected:  true,
		},
		{
			name:      "不同小时不应运行",
			hour:      10,
			minute:    30,
			checkTime: time.Date(2024, 1, 1, 11, 30, 15, 0, time.Local),
			expected:  false,
		},
		{
			name:      "不同分钟不应运行",
			hour:      10,
			minute:    30,
			checkTime: time.Date(2024, 1, 1, 10, 31, 15, 0, time.Local),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := NewTask(tt.hour, tt.minute, func() {})
			result := task.ShouldRun(tt.checkTime)
			if result != tt.expected {
				t.Errorf("ShouldRun() = %v，期望 %v", result, tt.expected)
			}
		})
	}
}

func TestTaskNextRunTime(t *testing.T) {
	// 测试时间: 2024-01-15 09:00:00
	baseTime := time.Date(2024, 1, 15, 9, 0, 0, 0, time.Local)

	tests := []struct {
		name     string
		hour     int
		minute   int
		expected time.Time
	}{
		{
			name:     "今天的时间还未到",
			hour:     14,
			minute:   30,
			expected: time.Date(2024, 1, 15, 14, 30, 0, 0, time.Local),
		},
		{
			name:     "今天的时间已过",
			hour:     8,
			minute:   0,
			expected: time.Date(2024, 1, 16, 8, 0, 0, 0, time.Local),
		},
		{
			name:     "同小时但分钟未到",
			hour:     9,
			minute:   30,
			expected: time.Date(2024, 1, 15, 9, 30, 0, 0, time.Local),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := NewTask(tt.hour, tt.minute, func() {})
			result := task.NextRunTime(baseTime)
			if !result.Equal(tt.expected) {
				t.Errorf("NextRunTime() = %v，期望 %v", result, tt.expected)
			}
		})
	}
}

func TestTaskWithTimezone(t *testing.T) {
	// 创建纽约时区的任务
	nyLoc, _ := time.LoadLocation("America/New_York")
	task := NewTask(10, 0, func() {}, WithTimezone(nyLoc))

	// 使用纽约时间检查 (1月纽约是冬令时 UTC-5)
	// 纽约时间 10:00 对应 UTC 15:00
	nyTime := time.Date(2024, 1, 15, 10, 0, 0, 0, nyLoc)

	// 应该返回 true，因为是纽约时间 10:00
	if !task.ShouldRun(nyTime) {
		t.Error("在纽约时区的 10:00 应该执行")
	}

	// 纽约时间 10:00 对应 UTC 15:00，所以 UTC 15:00 转换到纽约时区应该是 10:00
	utcTime := time.Date(2024, 1, 15, 15, 0, 0, 0, time.UTC)
	if !task.ShouldRun(utcTime) {
		t.Error("UTC 15:00 (对应纽约 10:00) 应该执行")
	}

	// UTC 10:00 对应纽约 05:00，不应该执行
	utcMorning := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	if task.ShouldRun(utcMorning) {
		t.Error("UTC 10:00 (对应纽约 05:00) 不应该执行")
	}
}

func TestTaskString(t *testing.T) {
	task := NewTask(10, 30, func() {}, WithName("测试任务"))

	if task.String() != "测试任务" {
		t.Errorf("String() = %s，期望 测试任务", task.String())
	}
}

func TestTaskTimeString(t *testing.T) {
	task := NewTask(10, 30, func() {})

	if task.TimeString() != "10:30" {
		t.Errorf("TimeString() = %s，期望 10:30", task.TimeString())
	}
}

// ============== Scheduler 测试 ==============

func TestNewScheduler(t *testing.T) {
	s := NewScheduler()

	if s.tasks == nil {
		t.Error("tasks 不应为 nil")
	}
	if s.checkInterval != time.Minute {
		t.Errorf("checkInterval = %v，期望 1m", s.checkInterval)
	}
	if s.running {
		t.Error("新调度器不应处于运行状态")
	}
}

func TestSchedulerAddAndRemoveTask(t *testing.T) {
	s := NewScheduler()

	task := s.AddTaskQuick(10, 30, func() {})

	if s.TaskCount() != 1 {
		t.Errorf("TaskCount() = %d，期望 1", s.TaskCount())
	}

	// 测试获取任务
	gotTask, exists := s.GetTask(task.ID)
	if !exists {
		t.Error("GetTask 应返回 true")
	}
	if gotTask.Hour != 10 || gotTask.Minute != 30 {
		t.Error("获取的任务信息不正确")
	}

	// 测试移除任务
	if !s.RemoveTask(task.ID) {
		t.Error("RemoveTask 应返回 true")
	}
	if s.TaskCount() != 0 {
		t.Errorf("TaskCount() = %d，期望 0", s.TaskCount())
	}

	// 测试移除不存在的任务
	if s.RemoveTask("non-existent") {
		t.Error("移除不存在的任务应返回 false")
	}
}

func TestSchedulerListTasks(t *testing.T) {
	s := NewScheduler()

	s.AddTaskQuick(10, 0, func() {})
	s.AddTaskQuick(14, 30, func() {})
	s.AddTaskQuick(20, 15, func() {})

	tasks := s.ListTasks()
	if len(tasks) != 3 {
		t.Errorf("ListTasks() 返回 %d 个任务，期望 3", len(tasks))
	}
}

func TestSchedulerTaskExecution(t *testing.T) {
	s := NewSchedulerWithInterval(100 * time.Millisecond)

	var counter int64
	var mu sync.Mutex
	executed := false

	// 使用当前时间的下一分钟来测试
	now := time.Now()
	nextMinute := now.Add(2 * time.Minute)
	hour, minute := nextMinute.Hour(), nextMinute.Minute()

	task := NewTask(hour, minute, func() {
		mu.Lock()
		executed = true
		mu.Unlock()
		atomic.AddInt64(&counter, 1)
	}, WithID("test-task"), WithName("定时任务"))

	s.AddTask(task)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 启动调度器
	s.Start(ctx)

	// 等待足够长的时间以触发任务
	waitTime := 3 * time.Minute
	timeout := time.After(waitTime)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			cancel()
			s.Stop(0)
			t.Error("等待任务执行超时")
			return
		case <-ticker.C:
			mu.Lock()
			if executed {
				mu.Unlock()
				cancel()
				s.Stop(0)
				return // 测试通过
			}
			mu.Unlock()
		}
	}
}

func TestSchedulerStartStop(t *testing.T) {
	s := NewSchedulerWithInterval(100 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	// 启动调度器
	s.Start(ctx)

	if !s.IsRunning() {
		t.Error("调度器应该处于运行状态")
	}

	// 立即停止
	cancel()
	s.Stop(0)

	if s.IsRunning() {
		t.Error("调度器应该已停止")
	}
}

func TestSchedulerDoubleStart(t *testing.T) {
	s := NewSchedulerWithInterval(100 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 尝试启动两次
	s.Start(ctx)
	s.Start(ctx) // 应该无效

	if !s.IsRunning() {
		t.Error("调度器应该处于运行状态")
	}

	s.Stop(0)
}

func TestSchedulerStatus(t *testing.T) {
	s := NewScheduler()

	s.AddTaskWithOptions(10, 0, func() {}, WithID("task1"), WithName("任务1"))
	s.AddTaskWithOptions(14, 30, func() {}, WithID("task2"), WithName("任务2"))

	status := s.Status()

	if status.Running {
		t.Error("未启动的调度器状态应为 false")
	}
	if status.TaskCount != 2 {
		t.Errorf("TaskCount = %d，期望 2", status.TaskCount)
	}
	if len(status.TaskList) != 2 {
		t.Errorf("TaskList 长度 = %d，期望 2", len(status.TaskList))
	}
}

func TestSchedulerWithLogger(t *testing.T) {
	s := NewScheduler()
	// 使用默认 logger，不 panic 即为通过
	s.Logger.Printf("测试日志: %s\n", "Hello")
}

// ============== 边界条件测试 ==============

func TestTaskMidnight(t *testing.T) {
	task := NewTask(0, 0, func() {}) // 00:00

	baseTime := time.Date(2024, 1, 15, 23, 59, 59, 0, time.Local)
	nextRun := task.NextRunTime(baseTime)

	expected := time.Date(2024, 1, 16, 0, 0, 0, 0, time.Local)
	if !nextRun.Equal(expected) {
		t.Errorf("NextRunTime() = %v，期望 %v", nextRun, expected)
	}
}

func TestTask23_59(t *testing.T) {
	task := NewTask(23, 59, func() {})

	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.Local)
	nextRun := task.NextRunTime(baseTime)

	expected := time.Date(2024, 1, 15, 23, 59, 0, 0, time.Local)
	if !nextRun.Equal(expected) {
		t.Errorf("NextRunTime() = %v，期望 %v", nextRun, expected)
	}
}

func TestSchedulerEmpty(t *testing.T) {
	s := NewSchedulerWithInterval(100 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	// 空调度器也应该能正常启动和停止
	s.Start(ctx)
	time.Sleep(200 * time.Millisecond)
	cancel()
	s.Stop(0)
}

func TestConcurrentTaskOperations(t *testing.T) {
	s := NewScheduler()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	var wg sync.WaitGroup
	numGoroutines := 10
	numTasks := 100

	// 并发添加任务
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numTasks; j++ {
				s.AddTaskQuick(10, 30, func() {})
			}
		}(i)
	}

	wg.Wait()

	// 并发移除任务
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tasks := s.ListTasks()
			for _, task := range tasks[:len(tasks)/2] {
				s.RemoveTask(task.ID)
			}
		}(i)
	}

	wg.Wait()
	s.Stop(0)

	// 不崩溃即通过
}

// ============== 性能测试 ==============

func BenchmarkTaskShouldRun(b *testing.B) {
	task := NewTask(10, 30, func() {})
	now := time.Date(2024, 1, 1, 10, 30, 0, 0, time.Local)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task.ShouldRun(now)
	}
}

func BenchmarkSchedulerAddTask(b *testing.B) {
	s := NewScheduler()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.AddTaskQuick(10, 30, func() {})
	}
}
