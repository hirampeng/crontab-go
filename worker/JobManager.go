package worker

import (
	"context"
	"log"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"

	"github.com/hirampeng/crontab-go/common"
)

type JobManager struct {
	client  *clientv3.Client
	kv      clientv3.KV
	lease   clientv3.Lease
	watcher clientv3.Watcher
}

var (
	G_jobManager *JobManager //单例
)

//初始化管理器
func InitJobManager() (err error) {
	//初始化配置
	config := clientv3.Config{
		Endpoints:   G_config.EtcdEndpoints,
		DialTimeout: time.Duration(G_config.EtcdDialTimeout) * time.Millisecond,
	}

	//建立连接
	client, err := clientv3.New(config)
	if err != nil {
		return err
	}

	G_jobManager = &JobManager{
		client:  client,
		kv:      client.KV,
		lease:   client.Lease,
		watcher: client.Watcher,
	}

	log.Println("初始化JobManager成功 ： ", *G_jobManager)

	//启动任务监听
	G_jobManager.watchJobs()
	log.Println("开始监听eccd key :", common.JOB_SAVE_DIR)

	//启动杀死任务的监听

	return nil
}

//监听任务变化
func (jobManager *JobManager) watchJobs() (err error) {
	var (
		jobEvent *common.JobEvent
	)
	//1 取所有job任务，获取当前集权的reversion
	getResponse, err := jobManager.kv.Get(context.TODO(), common.JOB_SAVE_DIR, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	//遍历获取当前目录下的所有任务
	for _, kvPair := range getResponse.Kvs {
		job, err := common.UnpackJob(kvPair.Value)
		if err == nil {
			jobEvent = common.BuildJobEvent(common.JOB_EVENT_SAVE, job)
			//把这个job同步给scheduler(调度协程)
			G_scheduler.PushJobEvent(jobEvent)
			log.Println(jobEvent)
		} else {
			log.Println(err)
			continue //任务格式不规范，先不处理
		}

	}
	//2 从该reversion开始监听变化事件
	go func() { //监听协程
		//从get时刻开始监听后续版本的变化
		watchStartRevision := getResponse.Header.Revision + 1
		//监听/cron/jobs/目录的后续变化
		watchChan := jobManager.watcher.Watch(context.TODO(), common.JOB_SAVE_DIR, clientv3.WithRev(watchStartRevision), clientv3.WithPrefix())
		//处理监听事件
		for watchResp := range watchChan {
			for _, watchEvent := range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT: //任务保存事件
					job, err := common.UnpackJob(watchEvent.Kv.Value)
					if err == nil {
						jobEvent = common.BuildJobEvent(common.JOB_EVENT_SAVE, job)
					}
					log.Println(jobEvent)
				case mvccpb.DELETE: //任务删除事件
					//反序列化Job, 推一个更新事件给scheduler
					delJobName := common.ExtractString(string(watchEvent.Kv.Key), common.JOB_SAVE_DIR)
					if len(delJobName) > 0 {
						jobEvent = common.BuildJobEvent(common.JOB_EVENT_DELETE, &common.Job{Name: delJobName})
					}
					log.Println(jobEvent)
				}
				//推给scheduler
				G_scheduler.PushJobEvent(jobEvent)

			}
		}
	}()
	return
}

//监听任务变化
func (jobManager *JobManager) watchKilledJobs() (err error) {
	var (
		jobEvent *common.JobEvent
	)
	//2 从该reversion开始监听变化事件
	go func() { //监听协程
		//从当前时刻开始监听是否有强杀的任务
		watchChan := jobManager.watcher.Watch(context.TODO(), common.JOB_KILL_DIR, clientv3.WithPrefix())
		//处理监听事件
		for watchResp := range watchChan {
			for _, watchEvent := range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT: //强杀任务事件
					//反序列化Job, 推一个强杀事件给scheduler
					killJobName := common.ExtractString(string(watchEvent.Kv.Key), common.JOB_KILL_DIR)
					if len(killJobName) > 0 {
						jobEvent = common.BuildJobEvent(common.JOB_EVENT_KILL, &common.Job{Name: killJobName})
					}
				case mvccpb.DELETE: //取消强杀  暂时不实现
					cancelKillJobName := common.ExtractString(string(watchEvent.Kv.Key), common.JOB_KILL_DIR)
					if len(cancelKillJobName) > 0 {
						//jobEvent = common.BuildJobEvent(common.JOB_EVENT_CANCEL_KILL, &common.Job{Name:cancelKillJobName})
						log.Println("取消强杀任务：", cancelKillJobName)
					}
				}
				//推给scheduler
				G_scheduler.PushJobEvent(jobEvent)

			}
		}
	}()
	return
}

//创建任务执行锁 抢到锁的才能执行
func (jobManager *JobManager) CreateJobLock(jobName string) (jobLock *JobLock) {
	jobLock = InitJobLock(jobName, G_jobManager.kv, G_jobManager.lease)
	return jobLock
}
