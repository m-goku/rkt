package rkt

import (
	"database/sql"

	"go.mongodb.org/mongo-driver/mongo"
)

type initPaths struct {
	rootPath    string
	folderNames []string
}

type cookieConfig struct {
	name     string
	lifetime string
	persists string
	secure   string
	domain   string
}

type databaseConfig struct {
	dsn      string
	database string
}

type Database struct {
	DataType string
	Pool     *sql.DB
	Conn     *mongo.Client
}

type redisConfig struct {
	host     string
	password string
	prefix   string
}
