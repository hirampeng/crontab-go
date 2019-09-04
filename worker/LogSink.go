package worker

import (
	"context"
	"fmt"
	"github.com/hirampeng/crontab-go/common"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//mongodb 存储日志
type LogSink struct {
	client         *mongo.Client
	logCollection  *mongo.Collection
	logChan        chan *common.JobLog
	autoCommitChan chan *common.LogBatch
}

var (
	G_logSink *LogSink
)

// 发送日志
func (logSink *LogSink) Append(jobLog *common.JobLog) {
	//logSink.logChan <- jobLog
	log.Println("logChan <---------------------------------- ", jobLog)
	select {
	case logSink.logChan <- jobLog:
	default: // 队列满了就丢弃
		log.Println("丢弃的日志：", *jobLog)
	}
}

// 日志存储协程
func (logSink *LogSink) writeLoop() {
	var (
		jobLog       *common.JobLog
		logBatch     *common.LogBatch // 当前的批次
		commitTimer  *time.Timer
		timeoutBatch *common.LogBatch // 超时批次
	)

	for {
		select {
		case jobLog = <-logSink.logChan:
			log.Println("接收到log日志：==================================", jobLog)
			if logBatch == nil {
				logBatch = &common.LogBatch{}
				// 让这个批次超时自动提交(给1秒的时间）
				commitTimer = time.AfterFunc(
					time.Duration(G_config.JobLogCommitTimeout)*time.Millisecond,
					func(batch *common.LogBatch) func() {
						return func() {
							logSink.autoCommitChan <- batch
						}
					}(logBatch),
				)
			}

			// 把新日志追加到批次中
			logBatch.JobLogs = append(logBatch.JobLogs, jobLog)

			// 如果批次满了, 就立即发送
			log.Println("批次是否满了：", len(logBatch.JobLogs), "  ", G_config.JobLogBatchSize)
			if len(logBatch.JobLogs) >= G_config.JobLogBatchSize {
				// 发送日志
				logSink.saveLogs(logBatch)
				// 清空logBatch
				logBatch = nil
				// 取消定时器
				commitTimer.Stop()
			}
		case timeoutBatch = <-logSink.autoCommitChan: // 过期的批次
			// 判断过期批次是否仍旧是当前的批次
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

// 批量写入日志
func (logSink *LogSink) saveLogs(batch *common.LogBatch) {
	log.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
	log.Println("插入任务执行日志：", batch.JobLogs)
	log.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
	logSink.logCollection.InsertMany(context.TODO(), batch.JobLogs)
}

func InitLogSink() (err error) {
	// Set client options
	clientOptions := options.Client().ApplyURI(G_config.MongodbUri).
		SetConnectTimeout(time.Duration(G_config.MongodbConnectTimeout) * time.Millisecond)

	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil {
		log.Fatal(err)
	}

	// Check the connection
	err = client.Ping(context.TODO(), nil)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB!")

	collection := client.Database("cron").Collection("log")

	G_logSink = &LogSink{
		client:        client,
		logCollection: collection,
		logChan:       make(chan *common.JobLog, 1000),
	}

	// 启动一个mongodb处理协程
	go G_logSink.writeLoop()

	return
}
