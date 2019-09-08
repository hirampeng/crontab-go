package worker

import (
	"context"
	"github.com/hirampeng/crontab-go/common"
	"go.etcd.io/etcd/clientv3"
	"log"
	"net"
	"time"
)

type Register struct {
	client *clientv3.Client
	kv     clientv3.KV
	lease  clientv3.Lease

	localIp string
}

var (
	G_register *Register
)

func InitRegister() (err error) {

	config := clientv3.Config{
		Endpoints:   G_config.EtcdEndpoints,
		DialTimeout: time.Duration(G_config.EtcdDialTimeout) * time.Millisecond,
	}

	client, err := clientv3.New(config)
	if err != nil {
		return err
	}

	G_register = &Register{
		client:  client,
		kv:      client.KV,
		lease:   client.Lease,
		localIp: "",
	}

	ipv4Address, err := getLocalIPv4Address()
	if err != nil {
		return err
	}

	G_register.localIp = ipv4Address

	go G_register.registerEtcdAndKeepOnline()

	return nil
}

func getLocalIPv4Address() (ipv4Address string, err error) {
	//获取所有网卡
	addrs, err := net.InterfaceAddrs()

	//遍历
	for _, addr := range addrs {
		//取网络地址的网卡的信息
		ipNet, isIpNet := addr.(*net.IPNet)
		//是网卡并且不是本地环回网卡
		if isIpNet && !ipNet.IP.IsLoopback() {
			ipv4 := ipNet.IP.To4()
			//能正常转成ipv4
			if ipv4 != nil {
				return ipv4.String(), nil
			}
		}
	}

	return
}

func (register *Register) registerEtcdAndKeepOnline() {

	for {
		//注册的key
		ipv4Address, err := getLocalIPv4Address()
		if err == nil { //刷新ip
			G_register.localIp = ipv4Address
		}

		cancelCtx, cancelFunc := context.WithCancel(context.TODO())
		registerKey := common.JOB_WORKER_DIR + register.localIp
		log.Println(registerKey)
		//创建租约
		grantResponse, err := register.lease.Grant(context.TODO(), 10)
		if err == nil {
			//自动续租
			keepAliveResponseChan, err := register.lease.KeepAlive(cancelCtx, grantResponse.ID)

			if err == nil {
				//注册到etcd
				_, err := register.kv.Put(cancelCtx, registerKey, "", clientv3.WithLease(grantResponse.ID))
				if err == nil {
					//处理续租应答
					for {
						select {
						case keepAliveResponse, isClose := <-keepAliveResponseChan:
							if keepAliveResponse == nil || isClose {
								goto RETRY
							}
						}
					}
				}
			}
		}

	RETRY:
		//上面处理出问题了，睡一会儿再试
		time.Sleep(3 * time.Second)
		if cancelFunc != nil {
			cancelFunc()
		}
	}
}
