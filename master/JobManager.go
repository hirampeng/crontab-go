package master

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"go.etcd.io/etcd/clientv3"

	"github.com/hirampeng/crontab-go/common"
)

type JobManager struct {
	client *clientv3.Client
	kv     clientv3.KV
	lease  clientv3.Lease
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
		client: client,
		kv:     client.KV,
		lease:  client.Lease,
	}

	log.Println("初始化JobManager成功 ： ", *G_jobManager)

	return nil
}

func (jobManager *JobManager) SaveJob(job *common.Job) (oldJob *common.Job, err error) {
	log.Println("保存的job信息:", *job)
	//定义etcd保存的key
	jobKey := common.JOB_SAVE_DIR + job.Name
	//任务信息转换成string
	bytes, err := json.Marshal(job)
	if err != nil {
		return
	}
	jobValue := string(bytes)

	log.Println("要保存的jobKey:", jobKey, "jobValue:", jobValue)
	log.Println(jobManager.kv)
	//保存在etcd
	putResponse, err := jobManager.kv.Put(context.TODO(), jobKey, jobValue, clientv3.WithPrevKV())
	if err != nil {
		return
	}

	if putResponse.PrevKv != nil {
		err = json.Unmarshal(putResponse.PrevKv.Value, &oldJob)
		if err != nil {
			return
		}
	}
	return oldJob, nil
}

func (jobManager *JobManager) DeleteJob(jobName string) (oldJob *common.Job, err error) {
	jobKey := common.JOB_SAVE_DIR + jobName
	deleteResponse, err := jobManager.kv.Delete(context.TODO(), jobKey, clientv3.WithPrevKV())
	if err != nil {
		return nil, err
	}

	if len(deleteResponse.PrevKvs) > 0 {
		jobValueBytes := deleteResponse.PrevKvs[0].Value
		err = json.Unmarshal(jobValueBytes, &oldJob)

		log.Println("删除任务成功,job信息：", oldJob)
		return oldJob, nil
	}

	return nil, nil
}

//获取etcd的列表
func (jobManager *JobManager) List() (jobs []*common.Job, err error) {
	getResponse, err := jobManager.kv.Get(context.TODO(), common.JOB_SAVE_DIR, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	jobs = make([]*common.Job, len(getResponse.Kvs))
	//初始化列表
	for index, kvPair := range getResponse.Kvs {
		job := &common.Job{}
		log.Println(common.JOB_SAVE_DIR)
		log.Println(string(kvPair.Value))
		err = json.Unmarshal(kvPair.Value, job)
		if err != nil {
			log.Println("获取列表失败：", err.Error())
			return nil, err
		}
		jobs[index] = job
	}
	return jobs, nil
}

//通知杀死任务
func (jobManager *JobManager) KillJob(killJobName string) (err error) {
	killJobKey := common.JOB_KILL_DIR + killJobName

	//让worker监听一次put操作，创建一个租约让他自动过期
	//ETCDCTL_API=3 ./etcdctl watch "/cron/killJobList/" --prefix
	leaseGrantResponse, err := jobManager.lease.Grant(context.TODO(), 1)
	if err != nil {
		return err
	}
	//put 操作  保存一秒过期
	_, err = jobManager.kv.Put(context.TODO(), killJobKey, "", clientv3.WithLease(leaseGrantResponse.ID))
	log.Println("删除job成功", killJobName)
	return err
}
