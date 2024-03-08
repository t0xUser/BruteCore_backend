package lib_env

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Config struct {
	App struct {
		Name      string `env:"APP_NAME"`
		Version   string `env:"APP_VERSION"`
		UseLogger bool   `env:"APP_USE_LOGGER"`
	}

	Auth struct {
		AdminLogin     string `env:"AUTH_ADMIN_LOGIN"`
		AdminPassword  string `env:"AUTH_ADMIN_PASSWORD"`
		ViewerLogin    string `env:"AUTH_VIEWER_LOGIN"`
		ViewerPassword string `env:"AUTH_VIEWER_PASSWORD"`
	}

	Http struct {
		Port  string `env:"HTTP_PORT" envDefault:":985"`
		Group string `env:"HTTP_GROUP"`
	}

	Jwt struct {
		Key         string        `env:"JWT_KEY"`
		Method      string        `env:"JWT_METHOD"`
		AccessTime  time.Duration `env:"JWT_ACCESS_TIME"`
		RefreshTime time.Duration `env:"JWT_REFRESH_TIME"`
	}

	Database struct {
		Path string `env:"DATABASE_PATH"`
	}

	Logs struct {
		DB     *os.File `env:"-"`
		Http   *os.File `env:"-"`
		Common *os.File `env:"-"`
	}
}

const (
	logDateFormat = "02.01.2006_15.04.05"
)

func fullFillStruct(cfg interface{}) error {
	configValue := reflect.ValueOf(cfg).Elem()
	configType := configValue.Type()

	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		fieldValue := configValue.Field(i)

		if field.Type.Kind() == reflect.Struct {
			if err := env.Parse(fieldValue.Addr().Interface()); err != nil {
				return err
			}
		}
	}

	return nil
}

func New() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	var conf Config
	if err := fullFillStruct(&conf); err != nil {
		return nil, err
	}

	dirName := filepath.Join(filepath.Dir(os.Args[0]), "logs")
	if err := createLogDirectory(dirName); err != nil {
		return nil, err
	}
	
	if conf.App.UseLogger {
		for _, val := range [3]string{"db", "http", "common"} {
			file, err := createLogfile(dirName, val)
			if err != nil {
				return nil, err
			}

			switch val {
			case "db":
				conf.Logs.DB = file
			case "http":
				conf.Logs.Http = file
			case "common":
				conf.Logs.Common = file
			}

		}
	}

	return &conf, nil
}

func (this *Config) Close() {
	if this.App.UseLogger {
		this.Logs.DB.Close()
		this.Logs.Http.Close()
		this.Logs.Common.Close()
	}
}

func createLogDirectory(dirName string) error {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		return os.MkdirAll(dirName, 0755)
	}
	return nil
}

func createLogfile(dirName, prefix string) (*os.File, error) {
	fileName := fmt.Sprintf("%s_logfile_%s.log", prefix, time.Now().Format(logDateFormat))
	return os.OpenFile(filepath.Join(dirName, fileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
}
