package config

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewGorm(config *viper.Viper, log *logrus.Logger) *gorm.DB {
	username := config.GetString("DATABASE_USERNAME")
	password := config.GetString("DATABASE_PASSWORD")
	host := config.GetString("DATABASE_HOST")
	port := config.GetInt("DATABASE_PORT")
	database := config.GetString("DATABASE_NAME")
	sslmode := config.GetString("DATABASE_SSLMODE")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		host, username, password, database, port, sslmode)

	var db *gorm.DB
	var sqlDB *sql.DB
	var err error

	gormLogLevel := logger.Warn
	if config.GetString("LOG_LEVEL") == "debug" {
		gormLogLevel = logger.Info
	}

	for i := range 10 {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.New(&logrusWriter{Log: log}, logger.Config{
				SlowThreshold:             200 * time.Millisecond,
				Colorful:                  false,
				IgnoreRecordNotFoundError: true,
				ParameterizedQueries:      true,
				LogLevel:                  gormLogLevel,
			}),
			TranslateError: true,
		})

		if err != nil {
			log.Printf("Waiting for database... attempt %d/10 | Error: %v", i+1, err)
			time.Sleep(3 * time.Second)
			continue
		}

		sqlDB, err = db.DB()
		if err != nil {
			log.Printf("Waiting for database... attempt %d/10 | Error: %v", i+1, err)
			time.Sleep(3 * time.Second)
			continue
		}

		err = sqlDB.Ping()
		if err != nil {
			_ = sqlDB.Close()
			log.Printf("Waiting for database... attempt %d/10 | Error: %v", i+1, err)
			time.Sleep(3 * time.Second)
			continue
		}

		break
	}

	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(1 * time.Minute)

	return db
}

type logrusWriter struct {
	Log *logrus.Logger
}

func (l *logrusWriter) Printf(message string, args ...any) {
	l.Log.Tracef(message, args...)
}
