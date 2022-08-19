package worker

import (
	"context"
	"fmt"
	"github.com/NetLops/cocrontab/common"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

// mongoDB存储日志

type LogSink struct {
	client        *mongo.Client
	logCollection *mongo.Collection
	logChan       chan *common.JobLog
	autoChan      chan *common.LogBatch // 超时通知
}

var (
	G_LogSink *LogSink
)

func (logSink LogSink) saveLogs(batch *common.LogBatch) {
	if _, err := logSink.logCollection.InsertMany(context.TODO(), batch.Logs); err != nil {
		fmt.Println("日志刷新失败:", err)
	}
}

// 日志存储协程
func (logSink *LogSink) writeLoop() {
	var (
		log         *common.JobLog
		logBatch    *common.LogBatch // 当前的批次
		commitTimer *time.Timer
	)

	for {
		select {
		case log = <-logSink.logChan:
			// 每次插入需要等待mongodb的一次请求往返，耗时可能因为网络慢花费比较长的时间

			if logBatch == nil {
				logBatch = &common.LogBatch{}
				// 让这个批次超时自动提交（1秒的暑假见）
				commitTimer = time.AfterFunc(time.Duration(G_config.JobLogCommitTimeout)*time.Millisecond,
					// 发出超时通知，不能直接提交，因为这个回调函数会启新协程会触发同步问题，用chan转换为串行
					func(logBatch *common.LogBatch) func() {
						return func() {
							logSink.autoChan <- logBatch
						}
					}(logBatch)) // 因为logBatch 可能会被nil，所以 用原来的上下文可能有问题，所以直接传进来
			}

			// 把新的日志追加到批次中
			logBatch.Logs = append(logBatch.Logs, log)

			// 如果批次满了， 就立即发送
			if len(logBatch.Logs) >= G_config.JobLogBatchSize {
				// 发送日志
				logSink.saveLogs(logBatch)
				// 发送完就直接扔了
				logBatch = nil // 此时该批次已经变了
				// 关闭定时器，取消定时器
				commitTimer.Stop()
			}
		case timeoutBatch := <-logSink.autoChan: // 过期的批次
			// 判断过期批次是否依旧是当前的批次
			if timeoutBatch != logBatch {
				continue // 跳过已经被提交的批次
			}
			// 把批次写入到mongo中
			logSink.saveLogs(timeoutBatch)

			// 清空logBatch
			logBatch = nil
		}
	}

}

// 发送日志
func (logSink *LogSink) Append(jobLog *common.JobLog) {
	select {
	case logSink.logChan <- jobLog:
	default:
		// 队列满了就丢弃
	}
}

func InitLogSink() (err error) {
	var (
		client        *mongo.Client
		clientOptions *options.ClientOptions
	)
	clientOptions = &options.ClientOptions{}
	clientOptions.Auth = &options.Credential{
		Username: G_config.MongoDBUsername,
		Password: G_config.MongoDBPassword,
	}
	clientOptions.SetConnectTimeout(time.Millisecond * time.Duration(G_config.MongoDBTimeout))
	clientOptions.ApplyURI(G_config.MongoDBUri)
	//  建立mongoDB地址
	if client, err = mongo.Connect(context.TODO(), clientOptions); err != nil {
		fmt.Println(err)
		return
	}

	// 选择DB
	G_LogSink = &LogSink{
		client:        client,
		logCollection: client.Database("cron").Collection("log"),
		logChan:       make(chan *common.JobLog, 1000),
		autoChan:      make(chan *common.LogBatch, 1000),
	}

	// 启动一个mongodb 处理协程
	go G_LogSink.writeLoop()

	return

}
