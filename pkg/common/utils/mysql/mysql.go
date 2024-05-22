package mysql

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"k8s.io/klog/v2"
)

type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Database string
}

type DB struct {
	*sqlx.DB
}

func NewDorisSqlDB(cfg DBConfig) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	db, err := sqlx.Open("mysql", dsn)
	if err != nil {
		klog.Errorf("failed open doris sql client connection, err: %s \n", err)
		return nil, err
	}

	if err = db.Ping(); err != nil {
		klog.Errorf("failed ping doris sql client connection, err: %s\n", err.Error())
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	klog.Infof("doris-operator exec sql: %s \n", query)
	return db.DB.Exec(query, args...)
}

func (db *DB) Select(dest interface{}, query string, args ...interface{}) error {
	klog.Infof("doris-operator exec select sql: %s \n", query)
	return db.DB.Select(dest, query, args...)
}
