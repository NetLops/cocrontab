package worker

import (
	"context"
	"fmt"
	"github.com/NetLops/cocrontab/common"
	"github.com/robfig/cron/v3"
	"time"
)

// 任务调度
type Scheduler struct {
	jobEventChan chan *common.JobEvent // etcd 任务事件队列
	//jobPlanTable map[string]*common.JobSchedulePlan // 任务调度计划表
	jobPlanCron        *cron.Cron
	jobNameToCronIDMap map[string]cron.EntryID
	jobResultChan      chan *JobExecuteResult // 回传结果队列
}

var (
	G_Scheduler *Scheduler
)

//func (scheduler *Scheduler) TrySchedule() (scheduleAfter time.Duration) {
//	var (
//		jobPlan *common.JobSchedulePlan
//		now     time.Time
//	)
//
//	// 当前时间
//	now = time.Now()
//
//	// 遍历所有任务
//	for _, jobPlan := range scheduler.jobPlanTable {
//
//	}
//
//}

// 尝试执行任务
func (scheduler *Scheduler) TryStartJob(jobPlan *JobSchedulePlan) {

}

// 处理任务事件
func (scheduler *Scheduler) handleJobEvent(jobEvent *common.JobEvent) {
	var (
		jobSchedulePlan *JobSchedulePlan
		jobExisted      bool // 强杀出现的
		err             error
	)

	switch jobEvent.EventType {
	case common.JOB_EVENT_SAVE: // 保存任务事件
		if jobSchedulePlan, err = BuildJobSchedulePlain(jobEvent.Job); err != nil {
			return
		}
		// 判断任务是否已经存在 已经存在先移除再拉入
		if cronId, ok := scheduler.jobNameToCronIDMap[jobEvent.Job.Name]; ok {
			scheduler.jobPlanCron.Remove(cronId)
			delete(scheduler.jobNameToCronIDMap, jobEvent.Job.Name)
		}
		if jobSchedulePlan.CronId, err = scheduler.jobPlanCron.AddJob(jobSchedulePlan.Expr, jobSchedulePlan); err != nil {
			return
		}
		jobSchedulePlan.GetEntityByEntityId = func() cron.Entry {
			return scheduler.jobPlanCron.Entry(jobSchedulePlan.CronId)
		}
		jobSchedulePlan.PlanTime = jobSchedulePlan.GetEntityByEntityId().Next
		scheduler.jobNameToCronIDMap[jobSchedulePlan.Job.Name] = jobSchedulePlan.CronId
	case common.JOB_EVENT_DELETE: // 删除任务事件
		if cronId, ok := scheduler.jobNameToCronIDMap[jobEvent.Job.Name]; ok {
			scheduler.jobPlanCron.Remove(cronId)
			delete(scheduler.jobNameToCronIDMap, jobEvent.Job.Name)
		}
	case common.JOB_EVENT_KILL: // 强杀任务事件
		// 取消掉Command执行，判断任务是否在执行中
		if cronId, ok := scheduler.jobNameToCronIDMap[jobEvent.Job.Name]; ok {
			if jobSchedulePlan, jobExisted = scheduler.jobPlanCron.Entry(cronId).Job.(*JobSchedulePlan); jobExisted {
				jobSchedulePlan.CancelCtxCancelFunc() // 触发command 杀死子进程, 任务得到退出
				fmt.Println("子进程死亡")
			}
		}
	}
}

// 处理任务结果
// 记得调度完成 呜呜// result.ExecutorInfo.Running 会影响调度
func (scheduler *Scheduler) handleJobResult(result *JobExecuteResult) {
	var (
		jobLog *common.JobLog
	)
	// 被锁占用就直接跳过了
	if result.Err == common.ERR_LOCK_ALREADY_REQUIRED || (result.Err != nil && result.Err.Error() == "context canceled") {
		result.ExecutorInfo.Running = false // 本次调度完成
		result.ExecutorInfo.PlanTime = result.ExecutorInfo.GetEntityByEntityId().Next
		return
	}
	jobLog = &common.JobLog{
		JobName: result.ExecutorInfo.Job.Name,
		Command: result.ExecutorInfo.Job.Command,
		//Output:       string(),
		PlanTime:     result.ExecutorInfo.PlanTime.UnixMilli(),
		ScheduleTime: result.ExecutorInfo.RealTime.UnixMilli(),
		StartTime:    result.StartTime.UnixMilli(),
		EndTime:      result.EndTime.UnixMilli(),
	}
	if result.Output != nil {
		jobLog.Output = string(result.Output)
	}
	if result.Err == nil {
		fmt.Println("任务执行完成", string(result.Output))
		jobLog.Err = ""
	} else {
		fmt.Println("任务执行失败", result.Err)
		jobLog.Err = result.Err.Error()
	}
	result.ExecutorInfo.Running = false       // 本次调度完成
	fmt.Println(result.ExecutorInfo.PlanTime) // 获取计划调度时间
	fmt.Println(result.ExecutorInfo.RealTime) // 真实执行的时间
	result.ExecutorInfo.PlanTime = result.ExecutorInfo.GetEntityByEntityId().Next
	G_LogSink.Append(jobLog) // 日志防止堵塞，写不进去会被丢
}

// 调度协程
func (scheduler *Scheduler) scheduleLoop() {
	defer func() {
		scheduler.jobPlanCron.Stop()
		close(scheduler.jobEventChan)
	}()
	var (
		jobEvent  *common.JobEvent
		jobResult *JobExecuteResult
	)

	go func() {
		scheduler.jobPlanCron.Run()
	}()
	for {
		select {
		case jobEvent = <-scheduler.jobEventChan: // 监听任务变化时间
			// 对内存中任务列表做增删改查
			scheduler.handleJobEvent(jobEvent)
		case jobResult = <-scheduler.jobResultChan: // 监听任务执行结果
			scheduler.handleJobResult(jobResult)

		}
	}
}

// 推送任务变化事件
func (scheduler *Scheduler) PushJobEvent(jobEvent *common.JobEvent) {
	scheduler.jobEventChan <- jobEvent
}

// 初始化调度器
func InitScheduler() (err error) {
	G_Scheduler = &Scheduler{
		jobEventChan: make(chan *common.JobEvent, 1000), // 1000 个调度容量
		//jobPlanTable: map[string]*common.JobSchedulePlan{},
		jobPlanCron:        cron.New(cron.WithSeconds()), // 按秒来调度
		jobNameToCronIDMap: map[string]cron.EntryID{},
		jobResultChan:      make(chan *JobExecuteResult, 1000),
	}
	// 启动调度协程
	go G_Scheduler.scheduleLoop()
	return
}

// 回传任务执行结果
func (scheduler Scheduler) PushJobResult(jobResult *JobExecuteResult) {
	scheduler.jobResultChan <- jobResult
}

// 任务调度计划
type JobSchedulePlan struct {
	Job                 *common.Job // 要调度的任务信息
	CronId              cron.EntryID
	Expr                string    // Cron 表达式
	Once                bool      // 是否只执行一次
	Cancel              func()    // 这样才会被取消
	Running             bool      // 是否是运行的, 保证再一次运行中不会出现二次
	PlanTime            time.Time // 计划调度时间
	RealTime            time.Time // 真实调度时间
	CancelCtx           context.Context
	CancelCtxCancelFunc context.CancelFunc
	GetEntityByEntityId func() cron.Entry
}

type JobExecuteResult struct {
	ExecutorInfo *JobSchedulePlan
	Output       []byte    // 脚本输出
	Err          error     // 脚本错误原因
	StartTime    time.Time // 启动时间
	EndTime      time.Time // 结束时间
}

// 调度和执行是两件事情
// 实行的任务可能运行很久，1分钟会调度60次
func (j *JobSchedulePlan) Run() {
	if !j.Running {
		j.Running = true
	} else {
		return
	}

	/// 执行任务
	G_Executor.ExecuteJob(j) // 推送过去后会把 j.Running 置为false
	/// 执行任务结束

}

// 构造执行计划
func BuildJobSchedulePlain(job *common.Job) (jobSchedulePlan *JobSchedulePlan, err error) {
	var (
		expr string
	)
	expr = job.CronExpr

	//生成任务调度计划对象
	jobSchedulePlan = &JobSchedulePlan{
		Job:  job,
		Expr: expr,
	}
	jobSchedulePlan.CancelCtx, jobSchedulePlan.CancelCtxCancelFunc = context.WithCancel(context.TODO())
	return

}
