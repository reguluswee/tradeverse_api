package config

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	system "chaos/api/log"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type CmdConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type HttpConfig struct {
	Port int `yaml:"port"`
}

type IndexerRootConfig struct {
	Path string `yaml:"path"`
}

type ContractConfig struct {
	NAddress string `yaml:"nAddress"`
}

type Config struct {
	Database    DatabaseConfig    `yaml:"database"`
	Chain       []ChainConfig     `yaml:"chain"`
	Log         LogConfig         `yaml:"log"`
	AllStart    int               `yaml:"allStart"`
	Cmd         CmdConfig         `yaml:"cmd"`
	Http        HttpConfig        `yaml:"http"`
	ProxyEnable bool              `yaml:"proxyEnable"`
	IndexerRoot IndexerRootConfig `yaml:"indexerRoot"`
	Contract    ContractConfig    `yaml:"contract"`
}

// DatabaseConfig holds the database connection parameters.
type DatabaseConfig struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
	TimeZone string `yaml:"TimeZone"`
}

// ChainConfig holds the Solana chain RPC endpoints.
type ChainConfig struct {
	Name         string   `yaml:"name"`
	WsRpc        string   `yaml:"wsRpc"`
	QueryRpc     []string `yaml:"queryRpc"`
	SlotParallel int      `yaml:"slotParallel"`
	TxDetal      int      `yaml:"txDetal"`
	RangeRound   int      `yaml:"rangeRound"`
	Rpcs         []RpcMapper
	RpcMap       map[string]int
}

// LogConfig holds the logging directory and file name.
type LogConfig struct {
	Path string `yaml:"path"`
}

type RpcMapper struct {
	Rpc   string
	Quote int
}

func initRpcs(ts []ChainConfig) {
	for i := range ts {
		ts[i].initRpc()
	}
}

func (t *ChainConfig) initRpc() {
	if len(t.QueryRpc) == 0 {
		log.Fatal("error rpc config")
	}

	t.RpcMap = make(map[string]int)
	for _, r := range t.QueryRpc {
		v := strings.Split(r, "||")
		num := 0
		if len(v) == 2 {
			numStr := v[1]
			var err error
			num, err = strconv.Atoi(numStr)
			if err != nil {
				log.Fatal("error rpc inner format with Quote:", err)
			}
		}

		t.Rpcs = append(t.Rpcs, RpcMapper{
			Rpc:   v[0],
			Quote: num,
		})
		t.RpcMap[v[0]] = num
	}
}

func GetRpcConfig(code string) *ChainConfig {
	for _, v := range systemConfig.Chain {
		if v.Name == code {
			return &v
		}
	}
	return nil
}

func (t ChainConfig) GetRpc() []string {
	r := make([]string, 0)
	for _, v := range t.Rpcs {
		r = append(r, v.Rpc)
	}
	return r
}

func (t ChainConfig) GetRpcMapper() []RpcMapper {
	return t.Rpcs
}

func (t *ChainConfig) GetSlotParallel() int {
	if t.SlotParallel > 0 {
		return t.SlotParallel
	}
	return 1
}

func (t *ChainConfig) GetTxDelay() int {
	if t.TxDetal > 0 {
		return t.TxDetal
	}
	return 0
}

var systemConfig = &Config{}

func GetConfig() Config {
	return *systemConfig
}

func findProjectRoot(currentDir, rootIndicator string) (string, error) {
	if _, err := os.Stat(filepath.Join(currentDir, rootIndicator)); err == nil {
		return currentDir, nil
	}
	parentDir := filepath.Dir(currentDir)
	if currentDir == parentDir {
		return "", os.ErrNotExist
	}
	return findProjectRoot(parentDir, rootIndicator)
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}
	var confFilePath string

	if configFilePathFromEnv := os.Getenv("DALINK_GO_CONFIG_PATH"); configFilePathFromEnv != "" {
		confFilePath = configFilePathFromEnv
	} else {
		_, filename, _, _ := runtime.Caller(0)
		testDir := filepath.Dir(filename)
		confFilePath, _ = findProjectRoot(testDir, "__mark__")
		if len(confFilePath) > 0 {
			confFilePath += "/config/dev.yml"
		}
	}
	if len(confFilePath) == 0 {
		log.Fatal("System root directory setting error.")
	}
	log.Println("current config file ", confFilePath)

	viper.SetConfigFile(confFilePath)

	viper.SetConfigType("yml")
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Unable to read configuration file: %s", err)
	}

	err = viper.Unmarshal(&systemConfig)
	if err != nil {
		log.Fatalf("Unable to parse configuration: %s", err)
	}

	if len(systemConfig.Database.Host) == 0 {
		dbp := os.Getenv("DATABASE_HOST")
		log.Println("reset db host:", dbp)
		systemConfig.Database.Host = dbp
	}
	if len(systemConfig.Database.User) == 0 {
		dbp := os.Getenv("DATABASE_USER")
		log.Println("reset db user:", dbp)
		systemConfig.Database.User = dbp
	}
	if len(systemConfig.Database.Password) == 0 {
		dbp := os.Getenv("DATABASE_PWD")
		systemConfig.Database.Password = dbp
	}
	if systemConfig.Database.Port == 0 {
		dbp := os.Getenv("DATABASE_PORT")
		port, err := strconv.Atoi(dbp)
		if err != nil {
			log.Fatalf("invalid DATABASE.PORT value: %v", err)
		}
		systemConfig.Database.Port = port
	}
	initRpcs(systemConfig.Chain)

	system.InitLogger(systemConfig.Log.Path)

	//_ = godotenv.Load()

	system.Infof("initing default %s chain config", "Solana")
}
