package worker

import (
	"encoding/json"
	"io/ioutil"
)

// 程序配置
type Config struct {
	EtcdEndpoints       []string `json:"etcdEndpoints"`
	EtcdDialTimeout     int      `json:"etcdDialTimeout"`
	MongoDBUri          string   `json:"mongoDBUri"`          // mongoDB uri地址
	MongoDBUsername     string   `json:"mongoDBUsername"`     // mongoDB 用户名
	MongoDBPassword     string   `json:"mongoDBPassword"`     // mongoDB 密码
	MongoDBTimeout      int      `json:"mongoDBTimeout"`      // mongoDB 连接超时时间
	JobLogBatchSize     int      `json:"jobLogBatchSize"`     // mongoDB 需要打包的日志批次大小
	JobLogCommitTimeout int      `json:"jobLogCommitTimeout"` // mongoDB log 超时之前未提交会自动提交
}

var (
	G_config *Config
)

// 加载配置
func InitConfig(filename string) (err error) {
	var (
		content []byte
		conf    Config
	)

	// 把配置文件读进来
	if content, err = ioutil.ReadFile(filename); err != nil {
		return
	}

	// 反序列化
	if err = json.Unmarshal(content, &conf); err != nil {
		return
	}

	// 赋值单例
	G_config = &conf
	return
}
