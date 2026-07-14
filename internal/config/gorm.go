package config

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewGorm(config *viper.Viper, log *logrus.Logger) *gorm.DB {
	username := config.GetString("DATABASE_USERNAME")
	password := config.GetString("DATABASE_PASSWORD")
	host := config.GetString("DATABASE_HOST")
	port := config.GetInt("DATABASE_PORT")
	database := config.GetString("DATABASE_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", username, password, host, port, database)

	var db *gorm.DB
	var err error

	gormLogLevel := logger.Warn
	if config.GetString("LOG_LEVEL") == "debug" {
		gormLogLevel = logger.Info
	}

	for i := range 10 {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.New(&logrusWriter{Log: log}, logger.Config{
				SlowThreshold:             200 * time.Millisecond,
				Colorful:                  false,
				IgnoreRecordNotFoundError: true,
				ParameterizedQueries:      true,
				LogLevel:                  gormLogLevel,
			}),
		})

		if err != nil {
			log.Printf("Waiting for database... attempt %d/10", i+1)
			time.Sleep(3 * time.Second)
			continue
		}

		break
	}

	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}

	connection, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get database connection: %v", err)
	}

	connection.SetMaxOpenConns(25)
	connection.SetMaxIdleConns(10)
	connection.SetConnMaxLifetime(5 * time.Minute)
	connection.SetConnMaxIdleTime(1 * time.Minute)

	return db
}

type logrusWriter struct {
	Log *logrus.Logger
}

func (l *logrusWriter) Printf(message string, args ...any) {
	l.Log.Tracef(message, args...)
}