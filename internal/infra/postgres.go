package infra

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
)

var pgSingleton *gorm.DB

func InitPostgresql() *gorm.DB {

	dsn := os.Getenv("POSTGRES_URL")

	connectionPool, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Printf("Error connecting to database: %v", err)
		log.Fatal("Error connecting to database")
	}

	return connectionPool
}

func ClosePostgresql(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("Error getting database instance: %v", err)
		return
	}

	if err := sqlDB.Close(); err != nil {
		log.Printf("Error closing database connection: %v", err)
	} else {
		log.Println("PostgreSQL database connection closed successfully")
	}
}

func GetPostgresql() *gorm.DB {
	if pgSingleton == nil {
		log.Fatal("PostgreSQL database not initialized")
	}
	return pgSingleton
}

func StartTransaction(db *gorm.DB) *gorm.DB {
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
