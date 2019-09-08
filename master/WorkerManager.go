package master

import (
	"context"
	"github.com/hirampeng/crontab-go/common"
	"go.etcd.io/etcd/clientv3"
	"time"
)

//监听查看 cron/workers
type WorkerManager struct {
	client *clientv3.Client
	kv     clientv3.KV
	lease  clientv3.Lease
}

var (
	G_workerManager *WorkerManager
)

func InitWorkerManager() (err error) {
	config := clientv3.Config{
		Endpoints:   G_config.EtcdEndpoints,
		DialTimeout: time.Duration(G_config.EtcdDialTimeout) * time.Millisecond,
	}

	client, err := clientv3.New(config)
	if err != nil {
		return err
	}

	G_workerManager = &WorkerManager{
		client: client,
		kv:     client.KV,
		lease:  client.Lease,
	}
	return nil
}

//获取在线节点列表
func (workerManager *WorkerManager) ListWorkers() (workers []string, err error) {
	workers = make([]string, 0)

	//获取健康节点前缀的所有kv
	getResponse, err := workerManager.kv.Get(context.TODO(), common.JOB_WORKER_DIR, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	//遍历将健康节点加入列表
	for _, kv := range getResponse.Kvs {
		if kv.Key != nil {
			key := common.ExtractString(string(kv.Key), common.JOB_WORKER_DIR)
			workers = append(workers, key)
		}
	}

	return workers, nil
}
