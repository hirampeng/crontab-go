package worker

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

var (
	G_config *Config
)

//定义配置结构体
type Config struct {
	ApiPort               int      `json:"apiPort"`
	ApiReadTimeOut        int      `json:"apiReadTimeout"`
	ApiWriteTimeout       int      `json:"apiWriteTimeout"`
	EtcdEndpoints         []string `json:"etcdEndpoints"`
	EtcdDialTimeout       int      `json:"etcdDialTimeout"`
	MongodbUri            string   `json:"mongodbUri"`
	MongodbConnectTimeout int      `json:"mongodbConnectTimeout"`
	JobLogBatchSize       int      `json:"jobLogBatchSize"`
	JobLogCommitTimeout   int      `json:"jobLogCommitTimeout"`
}

//加载配置
func InitConfig(filename string) error {
	var (
		err    error
		config Config
	)

	//读取文件
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	//json反序列化
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return err
	}

	//赋值单例
	G_config = &config

	bytes, _ = json.Marshal(config)
	log.Println("初始化配置成功: ", string(bytes))
	return nil
}
