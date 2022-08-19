package common

import (
	"encoding/json"
	"strings"
)

// 定时任务
type Job struct {
	Name     string `json:"name"`     // 任务名
	Command  string `json:"command"`  // shell 命令
	CronExpr string `json:"cronExpr"` // cron 表达式
}

// HTTP 接口应答
type Response struct {
	Errno int         `json:"errno"`
	Msg   string      `json:"msg"`
	Data  interface{} `json:"data"`
}

// 变化事件
type JobEvent struct {
	EventType int // SAEV,DELETE
	Job       *Job
}

// 应答方法
func BuildResponse(errno int, msg string, data interface{}) (resp []byte, err error) {
	// 定义一个response
	var (
		response Response
	)

	response.Errno = errno
	response.Msg = msg
	if data != nil {
		response.Data = data
	}

	// 序列化json
	resp, err = json.Marshal(response)
	return resp, err
}

// 反序列化Job

func UnpackJob(value []byte) (ret *Job, err error) {
	var (
		job *Job
	)
	job = &Job{}

	if err = json.Unmarshal(value, job); err != nil {
		return
	}
	ret = job
	return

}

// 提取etcd的key中提取任务名
// 从/cron/jobs/job10抹掉/cron/jobs/
func ExtraJobName(jobKey string) string {
	return strings.TrimPrefix(jobKey, JOB_SAVE_DIR)
}

func ExtraKillerName(killerKey string) string {
	return strings.TrimPrefix(killerKey, JOB_KILLER_DIR)
}

// 任务变化事件有两种，删除任务/更新任务
func BuildJobEvent(eventType int, job *Job) (jobEvent *JobEvent) {
	return &JobEvent{
		EventType: eventType,
		Job:       job,
	}
}

// 任务执行日志
type JobLog struct {
	JobName      string `bson:"jobName"`      // 任务的名字
	Command      string `bson:"command"`      // 脚本命令
	Err          string `bson:"err"`          // 错误原因
	Output       string `bson:"output"`       // 脚本输出
	PlanTime     int64  `bson:"planTime"`     // 计划开始时间
	ScheduleTime int64  `bson:"scheduleTime"` // 世纪调度时间
	StartTime    int64  `bson:"startTime"`    // 任务执行开始时间
	EndTime      int64  `bson:"endTime"`      // 任务执行结束时间
}

// 日志批次
type LogBatch struct {
	Logs []interface{} // 多条日志
}
