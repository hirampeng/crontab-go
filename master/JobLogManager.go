package master

import (
	"context"
	"fmt"
	"github.com/hirampeng/crontab-go/common"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

type JobLogManager struct {
	client        *mongo.Client
	logCollection *mongo.Collection
}

var (
	G_jobLogManager *JobLogManager
)

func InitJobLogManager() (err error) {
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

	G_jobLogManager = &JobLogManager{
		client:        client,
		logCollection: collection,
	}

	return
}

//查看任务日志
func (jobLogManager JobLogManager) JobLogList(name string, skip int64, limit int64) (jobLogArr []*common.JobLog, err error) {
	jobLogFilter := &common.JobLogFilter{JobName: name}
	jobLogSort := &common.JobLogSort{StartTime: -1}

	//查询操作对象
	findOptions := options.FindOptions{
		Sort:  jobLogSort,
		Skip:  &skip,
		Limit: &limit,
	}

	cursor, err := jobLogManager.logCollection.Find(context.TODO(), jobLogFilter, &findOptions)
	if err != nil {
		return nil, err
	}

	//延迟释放游标
	defer cursor.Close(context.TODO())

	//初始化jobLog数组
	jobLogArr = make([]*common.JobLog, 0)

	for cursor.Next(context.TODO()) {
		jobLog := &common.JobLog{}

		if err := cursor.Decode(jobLog); err != nil {
			continue //不符合规范的日志 不处理
		}

		jobLogArr = append(jobLogArr, jobLog)
	}

	return jobLogArr, nil
}
