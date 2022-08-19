package worker

import (
	"context"
	"github.com/NetLops/cocrontab/common"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

// 任务管理器
type JobMgr struct {
	client  *clientv3.Client
	kv      clientv3.KV
	lease   clientv3.Lease
	watcher clientv3.Watcher
}

var (
	// 单例
	G_jobMgr *JobMgr
)

// 监听任务变化
func (jobMgr *JobMgr) WatchJobs() (err error) {
	var (
		getResp            *clientv3.GetResponse
		job                *common.Job
		watchStartRevision int64
		watchChan          clientv3.WatchChan
		watchResp          clientv3.WatchResponse
		jobEvent           *common.JobEvent
	)

	// get一下/cron/jobs/目录下的所有任务，并且获知当前集群的revision
	if getResp, err = jobMgr.kv.Get(context.TODO(), common.JOB_SAVE_DIR, clientv3.WithPrefix()); err != nil {
		return
	}

	// 当前有哪些任务
	for _, kv := range getResp.Kvs {
		// 反序列化json得到Job
		if job, err = common.UnpackJob(kv.Value); err == nil {
			jobEvent = common.BuildJobEvent(common.JOB_EVENT_SAVE, job)
			// TODO：把这个Job同步给scheduler(调度协程)
			//fmt.Println(*jobEvent)
			G_Scheduler.PushJobEvent(jobEvent)
		}
	}

	// 从该revision向后监听变化事件
	go func() { // 监听协程
		// 从GET时刻的后续版本
		watchStartRevision = getResp.Header.Revision + 1

		// 监听/cron/jobs/目录的后续便哈
		watchChan = jobMgr.watcher.Watch(context.TODO(), common.JOB_SAVE_DIR, clientv3.WithRev(watchStartRevision), clientv3.WithPrefix())

		// 处理监听事件
		for watchResp = range watchChan {
			for _, watchEvent := range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT: // 任务保存事件
					if job, err = common.UnpackJob(watchEvent.Kv.Value); err != nil {
						// 忽略本次事件
						continue
					}
					// 构建一个更新Event
					jobEvent = common.BuildJobEvent(common.JOB_EVENT_SAVE, job)

				case mvccpb.DELETE: // 任务被删除了
					// Delete /cron/jobs/job10
					jobName := common.ExtraJobName(string(watchEvent.Kv.Key))

					job = &common.Job{Name: jobName}
					// 构建一个删除Event
					jobEvent = common.BuildJobEvent(common.JOB_EVENT_DELETE, job)

				}
				//TODO 反序列化Job，推送一个更新事件给scheduler
				//TODO 推一个删除事件给scheduler
				G_Scheduler.PushJobEvent(jobEvent)
				//fmt.Println(*jobEvent)
			}
		}
	}()
	return
}

// 监听强杀任务通知
func (jobMgr *JobMgr) WatchKiller() {

	var (
		watchChan clientv3.WatchChan
		watchResp clientv3.WatchResponse
		jobEvent  *common.JobEvent
		jobName   string
		job       *common.Job
	)

	// 从该revision向后监听变化事件
	go func() { // 监听协程

		// 监听/cron/killer/目录的后续便哈
		watchChan = jobMgr.watcher.Watch(context.TODO(), common.JOB_KILLER_DIR, clientv3.WithPrefix())

		// 处理监听事件
		for watchResp = range watchChan {
			for _, watchEvent := range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT: // 杀死某个任务的事件
					jobName = common.ExtraKillerName(string(watchEvent.Kv.Key)) //cron/killer/job*
					job = &common.Job{Name: jobName}
					jobEvent = common.BuildJobEvent(common.JOB_EVENT_KILL, job)
					G_Scheduler.PushJobEvent(jobEvent)
				case mvccpb.DELETE: // killer标记过期，被自动删除
				}

				//fmt.Println(*jobEvent)
			}
		}
	}()
	return
}

// 初始化
func InitJobMgr() (err error) {
	var (
		config  clientv3.Config
		client  *clientv3.Client
		kv      clientv3.KV
		lease   clientv3.Lease
		watcher clientv3.Watcher
	)
	// 初始化配置
	config = clientv3.Config{
		Endpoints:   G_config.EtcdEndpoints,                                     // 集群地址
		DialTimeout: time.Duration(G_config.EtcdDialTimeout) * time.Millisecond, // 连接超时
	}

	// 建立连接
	if client, err = clientv3.New(config); err != nil {
		return

	}

	// 得到kv 和 lease 的API子集
	kv = clientv3.NewKV(client)
	lease = clientv3.NewLease(client)
	watcher = clientv3.NewWatcher(client)

	// 赋值单例
	G_jobMgr = &JobMgr{
		client:  client,
		kv:      kv,
		lease:   lease,
		watcher: watcher,
	}
	// 启动任务监听
	G_jobMgr.WatchJobs()

	// 启动监听killer
	G_jobMgr.WatchKiller()
	return

}
