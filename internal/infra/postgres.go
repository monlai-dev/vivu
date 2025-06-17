package infra

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
)

type PGDB struct {
	*gorm.DB
}

var pgSingleton *PGDB

func InitPostgresql() {

	dsn := os.Getenv("POSTGRES_URL")

	connectionPool, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Printf("Error connecting to database: %v", err)
		log.Fatal("Error connecting to database")
	}

	pgSingleton = &PGDB{
		DB: connectionPool,
	}
}

func ClosePostgresql() {
	if pgSingleton != nil && pgSingleton.DB != nil {
		sqlDB, err := pgSingleton.DB.DB()
		if err != nil {
			log.Printf("Error getting database instance: %v", err)
			return
		}
		if err := sqlDB.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}
}

func GetPostgresql() *PGDB {
	if pgSingleton == nil {
		log.Fatal("PostgreSQL database not initialized")
	}
	return pgSingleton
}

func StartTransaction() *gorm.DB {
	db := pgSingleton.DB
	tx := db.Begin()
	if tx.Error != nil {
		log.Printf("Error starting transaction: %v", tx.Error)
	}
	return tx
}

func ReleaseTransaction(tx *gorm.DB, err error) {
	if err != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			log.Printf("Error rollback transaction: %v", err)
		}
		return
	}
	if commitErr := tx.Commit().Error; commitErr != nil {
		log.Printf("Error committing transaction: %v", commitErr)
	} else {
		log.Println("Transaction committed successfully")
	}
}
