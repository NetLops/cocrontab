package worker

import (
	"context"
	"os/exec"
	"runtime"
	"time"
)

// 任务执行期
type Executor struct {
}

var (
	G_Executor *Executor
)

// 执行一个任务
func (e *Executor) ExecuteJob(info *JobSchedulePlan) {
	go func() {
		var (
			cmd    *exec.Cmd
			err    error
			output []byte
			result *JobExecuteResult
		)

		result = &JobExecuteResult{
			ExecutorInfo: info,
		}
		result.StartTime = time.Now()
		// 执行shell命令
		// 判断一手是那个系统
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(context.TODO(), "c:\\cygwin64\\bin\\bash.exe", "-c", info.Job.Command)
		} else {
			cmd = exec.CommandContext(context.TODO(), "/bin/bash", "-c", info.Job.Command)
		}

		// 执行并捕获输出
		output, err = cmd.CombinedOutput()

		// 记录任务结束时间
		result.EndTime = time.Now()

		result.Output = output
		result.Err = err
		// 任务执行完成后，把执行的结果返回给Scheduler, Scheduler
		result.ExecutorInfo.RealTime = time.Now()
		G_Scheduler.PushJobResult(result)
	}()
}

// 初始化执行器
func InitExecutor() (err error) {
	G_Executor = &Executor{}
	return
}
